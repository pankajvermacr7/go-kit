package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"reflect"
	"sync"
	"time"

	gormPostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file" // file driver for migrations
	"github.com/pankajvermacr7/gokit/logging"
	"github.com/rs/zerolog/log"
)

var (
	modelBuffer = make([]interface{}, 0) // Buffer to hold events
	bufferMutex sync.Mutex               // Mutex to protect the buffer
)

type DBConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	DBName          string
	MaxConns        int
	MinConns        int
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// BatchInsertConfig holds configuration for batch insert operations
type BatchInsertConfig struct {
	ConflictColumns []string
	DoNothing       bool
	Updates         map[string]interface{}
}

// ConflictOption defines a function type for configuring conflict handling
type ConflictOption func(*BatchInsertConfig)

var db *gorm.DB

// Connect establishes a connection to the database using the given config.
func Connect(cfg DBConfig) (*gorm.DB, error) {
	logger := logging.NewLogger()

	encodedPassword := url.QueryEscape(cfg.Password)
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s",
		cfg.Host, cfg.Port, cfg.User, encodedPassword, cfg.DBName)

	gormDB, err := gorm.Open(gormPostgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test the database connection
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get raw database instance: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxConns)
	sqlDB.SetMaxIdleConns(cfg.MinConns)
	sqlDB.SetConnMaxLifetime(cfg.MaxConnLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.MaxConnIdleTime)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db = gormDB
	logger.Info().Msg("Connected to database")
	return db, nil
}

// GetDB returns the connected database instance.
func GetDB() *gorm.DB {
	logger := logging.NewLogger()
	if db == nil {
		logger.Fatal().Msg("database connection is not initialized. Call Connect() first.")
	}
	return db
}

// WithOnConflictDoNothing specifies columns to watch for conflicts and do nothing on conflict
func WithOnConflictDoNothing(columns []string) ConflictOption {
	return func(c *BatchInsertConfig) {
		c.ConflictColumns = columns
		c.DoNothing = true
	}
}

// WithOnConflictDoUpdate specifies columns to watch for conflicts and perform updates
func WithOnConflictDoUpdate(columns []string, updates map[string]interface{}) ConflictOption {
	return func(c *BatchInsertConfig) {
		c.ConflictColumns = columns
		c.Updates = updates
	}
}

// ExecuteQueryWithContext allows executing custom SQL queries with a context-based timeout.
func ExecuteQueryWithContext(ctx context.Context, query string, args []interface{}) error {
	db := GetDB()

	if err := db.WithContext(ctx).Exec(query, args...).Error; err != nil {
		return fmt.Errorf("failed to execute custom query with context: %w", err)
	}
	return nil
}

// RunMigrations applies migrations from the specified directory.
func RunMigrations(cfg DBConfig, migrationsDir string) error {
	logger := logging.NewLogger()
	encodedPassword := url.QueryEscape(cfg.Password)
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s",
		cfg.Host, cfg.Port, cfg.User, encodedPassword, cfg.DBName)

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to open raw database connection: %w", err)
	}
	defer sqlDB.Close()

	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsDir),
		cfg.DBName,
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	logger.Info().Msg("Migrations applied successfully")
	return nil
}

// CreateTable creates a table in the database based on the given model.
func CreateTable(ctx context.Context, model interface{}, tableName ...string) error {
	db := GetDB()

	// If a custom table name is provided, set it
	if len(tableName) > 0 && tableName[0] != "" {
		db = db.WithContext(ctx).Table(tableName[0])
	}

	// Check if table already exists before creating
	if !IsTableExists(ctx, model) {
		if err := db.WithContext(ctx).AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to create table for model %T: %w", model, err)
		}
	}
	return nil
}

// IsTableExists checks if a table exists for a given model.
func IsTableExists(ctx context.Context, model interface{}, tableName ...string) bool {
	db := GetDB()

	// If a custom table name is provided, use it
	if len(tableName) > 0 && tableName[0] != "" {
		return db.WithContext(ctx).Migrator().HasTable(tableName[0])
	}

	// Otherwise, use the model's table name
	return db.WithContext(ctx).Migrator().HasTable(model)
}

// ExecuteCustomQuery allows executing custom SQL queries.
func ExecuteCustomQuery(ctx context.Context, query string, args ...interface{}) error {
	db := GetDB()
	if err := db.WithContext(ctx).Exec(query, args...).Error; err != nil {
		return fmt.Errorf("failed to execute custom query: %w", err)
	}
	return nil
}

// InsertData inserts a record into the specified table
func InsertData(ctx context.Context, record interface{}) error {
	db := GetDB()
	if err := db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("failed to insert record: %w", err)
	}
	return nil
}

// FetchData retrieves records from the specified table based on conditions.
func FetchData(ctx context.Context, model interface{}, conditions map[string]interface{}) error {
	db := GetDB()
	if err := db.WithContext(ctx).Where(conditions).Find(model).Error; err != nil {
		return fmt.Errorf("failed to fetch data: %w", err)
	}
	return nil
}

// UpdateData updates records in the specified table based on conditions.
func UpdateData(ctx context.Context, model interface{}, updates map[string]interface{}, conditions map[string]interface{}) error {
	db := GetDB()
	if err := db.WithContext(ctx).Model(model).Where(conditions).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update data: %w", err)
	}
	return nil
}

// DeleteData deletes records from the specified table based on conditions.
func DeleteData(ctx context.Context, model interface{}, conditions map[string]interface{}) error {
	db := GetDB()
	if err := db.WithContext(ctx).Where(conditions).Delete(model).Error; err != nil {
		return fmt.Errorf("failed to delete data: %w", err)
	}
	return nil
}

// If the buffer reaches the specified size, it flushes the buffer to the database.
func AddToBuffer(ctx context.Context, model interface{}, bufferSize int) error {
	bufferMutex.Lock()
	defer bufferMutex.Unlock()

	// Validate that the model is a struct
	if reflect.ValueOf(model).Kind() != reflect.Struct {
		return fmt.Errorf("model must be a struct, got %T", model)
	}

	// Add the model to the buffer
	modelBuffer = append(modelBuffer, model)

	// If buffer reaches the specified size, flush it to the database
	if len(modelBuffer) >= bufferSize {
		go func() {
			if err := FlushBufferToDB(ctx); err != nil {
				log.Error().Err(err).Msg("failed to flush buffer to DB")
			}
		}()
	}

	return nil
}

// FlushBufferToDB flushes the buffer to the database in a batch insert.
func FlushBufferToDB(ctx context.Context) error {
	bufferMutex.Lock()
	defer bufferMutex.Unlock()

	if len(modelBuffer) == 0 {
		return nil
	}

	// Get the database instance
	db := GetDB()

	// Perform batch insert
	if err := db.WithContext(ctx).Create(modelBuffer).Error; err != nil {
		return fmt.Errorf("failed to batch insert events: %w", err)
	}

	// Clear the buffer
	modelBuffer = nil
	return nil
}

// StartBufferFlusher starts a background goroutine to periodically flush the buffer to the database.
func StartBufferFlusher(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := FlushBufferToDB(ctx); err != nil {
			log.Error().Err(err).Msg("Failed to flush buffer to DB")
		}
	}
}

// BatchInsertData inserts multiple records in batches with optional conflict handling
// records must be a slice of structs, batchSize determines how many records per batch (0 = default)
func BatchInsertData(ctx context.Context, records interface{}, batchSize int, opts ...ConflictOption) error {
	if batchSize <= 0 {
		return fmt.Errorf("batchSize must be a positive integer")
	}
	db := GetDB()
	// Validate input type
	val := reflect.ValueOf(records)
	if val.Kind() != reflect.Slice {
		return fmt.Errorf("records must be a slice")
	}

	if val.Len() == 0 {
		return fmt.Errorf("empty slice provided: nothing to insert")
	}

	// Apply conflict options
	var conf BatchInsertConfig
	for _, opt := range opts {
		opt(&conf)
	}

	// Configure conflict handling if specified
	if len(conf.ConflictColumns) > 0 {
		onConflict := clause.OnConflict{
			Columns: make([]clause.Column, len(conf.ConflictColumns)),
		}

		for i, col := range conf.ConflictColumns {
			onConflict.Columns[i] = clause.Column{Name: col}
		}

		if conf.DoNothing {
			onConflict.DoNothing = true
		} else if len(conf.Updates) > 0 {
			onConflict.DoUpdates = clause.Assignments(conf.Updates)
		}

		db = db.Clauses(onConflict)
	}

	// Execute batch insert
	if err := db.CreateInBatches(records, batchSize).Error; err != nil {
		return fmt.Errorf("failed to perform batch insert: %w", err)
	}

	return nil
}
