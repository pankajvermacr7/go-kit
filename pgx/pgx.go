package pgx

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB holds the connection pool for PostgreSQL
type DB struct {
	pool *pgxpool.Pool
}

// Config holds database configuration parameters
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	Database string
	SSLMode  string
	MaxConns int
	Timeout  time.Duration
}

// NewDB creates a new database connection
func NewDB(cfg Config) (*DB, error) {
	if cfg.MaxConns <= 0 {
		cfg.MaxConns = 10 // Default connection pool size
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second // Default timeout
	}

	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s&pool_max_conns=%d",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database, cfg.SSLMode, cfg.MaxConns,
	)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("unable to parse connection string: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Close closes the database connection pool
func (db *DB) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
}

// GetPool returns the underlying pgx connection pool
func (db *DB) GetPool() *pgxpool.Pool {
	return db.pool
}

// Transaction provides a way to execute operations in a transaction
func (db *DB) Transaction(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p) // Re-throw panic after rollback
		}
	}()

	err = fn(tx)
	if err != nil {
		rbErr := tx.Rollback(ctx)
		if rbErr != nil {
			return fmt.Errorf("error rolling back transaction: %v, original error: %w", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ===== GENERIC CRUD OPERATIONS =====

// Get retrieves a single record by ID
func (db *DB) Get(ctx context.Context, dest interface{}, id interface{}) error {
	modelInfo, err := extractModelInfo(dest)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1 LIMIT 1",
		strings.Join(modelInfo.columnNames, ", "),
		modelInfo.tableName,
		modelInfo.idColumn,
	)

	row := db.pool.QueryRow(ctx, query, id)
	return scanStruct(row, dest)
}

// GetWhere retrieves records matching a condition
func (db *DB) GetWhere(ctx context.Context, dest interface{}, condition string, args ...interface{}) error {
	if reflect.TypeOf(dest).Kind() != reflect.Ptr || reflect.TypeOf(dest).Elem().Kind() != reflect.Slice {
		return errors.New("destination must be a pointer to a slice")
	}

	sliceVal := reflect.ValueOf(dest).Elem()
	elemType := sliceVal.Type().Elem()

	// Create a temporary instance to extract model info
	tempInstance := reflect.New(elemType).Interface()
	modelInfo, err := extractModelInfo(tempInstance)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s",
		strings.Join(modelInfo.columnNames, ", "),
		modelInfo.tableName,
		condition,
	)

	rows, err := db.pool.Query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		newElem := reflect.New(elemType).Interface()
		if err := scanStruct(rows, newElem); err != nil {
			return err
		}
		sliceVal.Set(reflect.Append(sliceVal, reflect.ValueOf(newElem).Elem()))
	}

	return rows.Err()
}

// Insert adds a new record to the database
func (db *DB) Insert(ctx context.Context, model interface{}) error {
	modelInfo, err := extractModelInfo(model)
	if err != nil {
		return err
	}

	placeholders := make([]string, len(modelInfo.columnNames))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
		modelInfo.tableName,
		strings.Join(modelInfo.columnNames, ", "),
		strings.Join(placeholders, ", "),
		modelInfo.idColumn,
	)

	var id interface{}
	err = db.pool.QueryRow(ctx, query, modelInfo.values...).Scan(&id)
	if err != nil {
		return fmt.Errorf("insert error: %w", err)
	}

	// Set the ID back to the model if it's settable
	if modelInfo.idField.IsValid() && modelInfo.idField.CanSet() {
		idVal := reflect.ValueOf(id)
		if idVal.Type().ConvertibleTo(modelInfo.idField.Type()) {
			modelInfo.idField.Set(idVal.Convert(modelInfo.idField.Type()))
		}
	}

	return nil
}

// Update updates an existing record
func (db *DB) Update(ctx context.Context, model interface{}) error {
	modelInfo, err := extractModelInfo(model)
	if err != nil {
		return err
	}

	if modelInfo.idValue == nil {
		return errors.New("cannot update model without ID value")
	}

	// Create set clauses (excluding ID)
	setClauses := make([]string, 0, len(modelInfo.columnNames)-1)
	updateValues := make([]interface{}, 0, len(modelInfo.columnNames))

	paramCounter := 1
	for i, col := range modelInfo.columnNames {
		if col != modelInfo.idColumn {
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, paramCounter))
			updateValues = append(updateValues, modelInfo.values[i])
			paramCounter++
		}
	}

	// Add ID as the last parameter
	updateValues = append(updateValues, modelInfo.idValue)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = $%d",
		modelInfo.tableName,
		strings.Join(setClauses, ", "),
		modelInfo.idColumn,
		paramCounter,
	)

	commandTag, err := db.pool.Exec(ctx, query, updateValues...)
	if err != nil {
		return fmt.Errorf("update error: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("no rows affected, record with ID %v may not exist", modelInfo.idValue)
	}

	return nil
}

// Delete removes a record from the database
func (db *DB) Delete(ctx context.Context, model interface{}) error {
	modelInfo, err := extractModelInfo(model)
	if err != nil {
		return err
	}

	if modelInfo.idValue == nil {
		return errors.New("cannot delete model without ID value")
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", modelInfo.tableName, modelInfo.idColumn)

	commandTag, err := db.pool.Exec(ctx, query, modelInfo.idValue)
	if err != nil {
		return fmt.Errorf("delete error: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("no rows affected, record with ID %v may not exist", modelInfo.idValue)
	}

	return nil
}

// ===== BATCH OPERATIONS =====

// BatchInsert inserts multiple records in a single transaction
func (db *DB) BatchInsert(ctx context.Context, models interface{}) error {
	sliceVal := reflect.ValueOf(models)
	if sliceVal.Kind() != reflect.Slice {
		return errors.New("models must be a slice")
	}

	if sliceVal.Len() == 0 {
		return nil // Nothing to insert
	}

	// Extract model info from the first element
	firstElem := sliceVal.Index(0).Interface()
	modelInfo, err := extractModelInfo(firstElem)
	if err != nil {
		return err
	}

	return db.Transaction(ctx, func(tx pgx.Tx) error {
		placeholders := make([]string, len(modelInfo.columnNames))
		for i := range placeholders {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}

		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
			modelInfo.tableName,
			strings.Join(modelInfo.columnNames, ", "),
			strings.Join(placeholders, ", "),
		)

		batch := &pgx.Batch{}

		// Add each model to the batch
		for i := 0; i < sliceVal.Len(); i++ {
			elem := sliceVal.Index(i).Interface()
			elemInfo, err := extractModelInfo(elem)
			if err != nil {
				return err
			}
			batch.Queue(query, elemInfo.values...)
		}

		results := tx.SendBatch(ctx, batch)
		defer results.Close()

		// Process all results
		for i := 0; i < batch.Len(); i++ {
			_, err := results.Exec()
			if err != nil {
				return fmt.Errorf("batch insert error at index %d: %w", i, err)
			}
		}

		return nil
	})
}

// BatchUpsert inserts or updates multiple records
func (db *DB) BatchUpsert(ctx context.Context, models interface{}, conflictColumns []string, updateColumns []string) error {
	sliceVal := reflect.ValueOf(models)
	if sliceVal.Kind() != reflect.Slice {
		return errors.New("models must be a slice")
	}

	if sliceVal.Len() == 0 {
		return nil // Nothing to upsert
	}

	// Extract model info from the first element
	firstElem := sliceVal.Index(0).Interface()
	modelInfo, err := extractModelInfo(firstElem)
	if err != nil {
		return err
	}

	return db.Transaction(ctx, func(tx pgx.Tx) error {
		// Build the base INSERT query
		placeholders := make([]string, len(modelInfo.columnNames))
		for i := range placeholders {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}

		// Create conflict resolution clause
		conflictClause := fmt.Sprintf("ON CONFLICT (%s) DO UPDATE SET ",
			strings.Join(conflictColumns, ", "))

		updateClauses := make([]string, 0, len(updateColumns))
		for _, col := range updateColumns {
			updateClauses = append(updateClauses, fmt.Sprintf("%s = EXCLUDED.%s", col, col))
		}

		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) %s%s",
			modelInfo.tableName,
			strings.Join(modelInfo.columnNames, ", "),
			strings.Join(placeholders, ", "),
			conflictClause,
			strings.Join(updateClauses, ", "),
		)

		batch := &pgx.Batch{}

		// Add each model to the batch
		for i := 0; i < sliceVal.Len(); i++ {
			elem := sliceVal.Index(i).Interface()
			elemInfo, err := extractModelInfo(elem)
			if err != nil {
				return err
			}
			batch.Queue(query, elemInfo.values...)
		}

		results := tx.SendBatch(ctx, batch)
		defer results.Close()

		// Process all results
		for i := 0; i < batch.Len(); i++ {
			_, err := results.Exec()
			if err != nil {
				return fmt.Errorf("batch upsert error at index %d: %w", i, err)
			}
		}

		return nil
	})
}

// ===== MIGRATION HELPERS =====

// RunMigration executes migration SQL scripts
func (db *DB) RunMigration(ctx context.Context, migrationSQL string) error {
	return db.Transaction(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, migrationSQL)
		if err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
		return nil
	})
}

// EnsureTable creates a table if it doesn't exist
func (db *DB) EnsureTable(ctx context.Context, model interface{}) error {
	modelInfo, err := extractModelInfo(model)
	if err != nil {
		return err
	}

	// Check if table exists
	var exists bool
	err = db.pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)",
		modelInfo.tableName).Scan(&exists)

	if err != nil {
		return fmt.Errorf("failed to check if table exists: %w", err)
	}

	if exists {
		return nil // Table already exists
	}

	// This is a simplified example - in a real implementation, you'd need to map
	// Go types to PostgreSQL types and handle more column attributes
	// For a complete solution, consider using a dedicated migration tool
	return fmt.Errorf("automatic table creation not implemented - please create migrations manually")
}

// ===== HELPER STRUCTS AND FUNCTIONS =====

type modelInfo struct {
	tableName   string
	columnNames []string
	values      []interface{}
	idColumn    string
	idField     reflect.Value
	idValue     interface{}
}

// extractModelInfo extracts metadata from a struct using reflection
func extractModelInfo(model interface{}) (*modelInfo, error) {
	modelType := reflect.TypeOf(model)
	modelValue := reflect.ValueOf(model)

	// Ensure we're working with a pointer to a struct
	if modelType.Kind() != reflect.Ptr {
		return nil, errors.New("model must be a pointer to a struct")
	}

	// Dereference the pointer
	modelType = modelType.Elem()
	modelValue = modelValue.Elem()

	if modelType.Kind() != reflect.Struct {
		return nil, errors.New("model must be a pointer to a struct")
	}

	// Get table name from struct name
	tableName := strings.ToLower(modelType.Name())

	var columnNames []string
	var values []interface{}
	var idColumn string
	var idField reflect.Value
	var idValue interface{}

	// Iterate through struct fields
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)
		value := modelValue.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		// Get column name from tag or field name
		columnName := field.Tag.Get("db")
		if columnName == "" {
			columnName = strings.ToLower(field.Name)
		}

		// Skip fields with "-" tag
		if columnName == "-" {
			continue
		}

		// Check if this is the ID field
		if field.Tag.Get("id") == "true" || columnName == "id" {
			idColumn = columnName
			idField = value
			if value.IsValid() && !value.IsZero() {
				idValue = value.Interface()
			}
		}

		columnNames = append(columnNames, columnName)
		values = append(values, value.Interface())
	}

	// If no ID column was found, default to "id"
	if idColumn == "" {
		idColumn = "id"
	}

	return &modelInfo{
		tableName:   tableName,
		columnNames: columnNames,
		values:      values,
		idColumn:    idColumn,
		idField:     idField,
		idValue:     idValue,
	}, nil
}

// scanStruct scans a row into a struct
func scanStruct(row pgx.Row, dest interface{}) error {
	modelInfo, err := extractModelInfo(dest)
	if err != nil {
		return err
	}

	destVal := reflect.ValueOf(dest).Elem()

	// Create a slice of pointers to hold scan destinations
	scanDest := make([]interface{}, len(modelInfo.columnNames))
	for i := range modelInfo.columnNames {
		field := destVal.FieldByNameFunc(func(name string) bool {
			field, _ := destVal.Type().FieldByName(name)
			dbTag := field.Tag.Get("db")
			return strings.ToLower(name) == modelInfo.columnNames[i] ||
				(dbTag != "" && dbTag == modelInfo.columnNames[i])
		})

		if field.IsValid() && field.CanSet() {
			scanDest[i] = field.Addr().Interface()
		} else {
			// If field not found, use a dummy destination
			var dummy interface{}
			scanDest[i] = &dummy
		}
	}

	return row.Scan(scanDest...)
}
