package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/gorm"
)

var testDBConfig DBConfig

type TestModel struct {
	ID    int    `gorm:"primaryKey"`
	Name  string `gorm:"type:varchar(100)"`
	Email string `gorm:"type:varchar(100);uniqueIndex"`
}

func setupPostgresContainer(t *testing.T) (testcontainers.Container, DBConfig) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:latest",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_DB":       "gokit",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}

	ip, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get PostgreSQL container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("Failed to get PostgreSQL container port: %v", err)
	}

	testDBConfig = DBConfig{
		Host:            ip,
		Port:            port.Int(),
		User:            "postgres",
		Password:        "postgres",
		DBName:          "gokit",
		MaxConns:        10,
		MinConns:        1,
		MaxConnLifetime: 5 * time.Minute,
		MaxConnIdleTime: 5 * time.Minute,
	}

	return container, testDBConfig
}

func teardownPostgresContainer(container testcontainers.Container) {
	container.Terminate(context.Background())
}

func cleanDatabase(t *testing.T, db *gorm.DB) {
	err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;").Error
	if err != nil {
		t.Fatalf("Failed to clean database: %v", err)
	}
}

func TestPostgresIntegration(t *testing.T) {
	container, config := setupPostgresContainer(t)
	defer teardownPostgresContainer(container)

	t.Run("TestConnect", func(t *testing.T) {
		db, err := Connect(config)
		assert.NoError(t, err)
		assert.NotNil(t, db)
		cleanDatabase(t, db)
	})

	t.Run("TestGetDB", func(t *testing.T) {
		_, err := Connect(config)
		assert.NoError(t, err)

		db := GetDB()
		assert.NotNil(t, db)
		cleanDatabase(t, db)
	})

	t.Run("TestRunMigrations", func(t *testing.T) {
		err := RunMigrations(config, "db/migrations")
		assert.NoError(t, err)
		cleanDatabase(t, db)
	})

	t.Run("TestCreateTable", func(t *testing.T) {
		ctx := context.Background()
		_, err := Connect(config)
		assert.NoError(t, err)

		err = CreateTable(ctx, &TestModel{})
		assert.NoError(t, err)
		cleanDatabase(t, db)
	})

	t.Run("TestInsertData", func(t *testing.T) {
		ctx := context.Background()
		_, err := Connect(config)
		assert.NoError(t, err)

		err = CreateTable(ctx, &TestModel{})
		assert.NoError(t, err)

		record := &TestModel{Name: "John Doe", Email: "john.doe@example.com"}
		err = InsertData(ctx, record)
		assert.NoError(t, err)
		cleanDatabase(t, db)
	})

	t.Run("TestFetchData", func(t *testing.T) {
		ctx := context.Background()
		_, err := Connect(config)
		assert.NoError(t, err)

		err = CreateTable(ctx, &TestModel{})
		assert.NoError(t, err)

		record := &TestModel{Name: "John Doe", Email: "john.doe@example.com"}
		err = InsertData(ctx, record)
		assert.NoError(t, err)

		var fetchedRecords []TestModel
		err = FetchData(ctx, &fetchedRecords, map[string]interface{}{"name": "John Doe"})
		assert.NoError(t, err)
		assert.Len(t, fetchedRecords, 1)
		assert.Equal(t, "John Doe", fetchedRecords[0].Name)
		assert.Equal(t, "john.doe@example.com", fetchedRecords[0].Email)

		cleanDatabase(t, db)
	})

	t.Run("TestUpdateData", func(t *testing.T) {
		ctx := context.Background()
		_, err := Connect(config)
		assert.NoError(t, err)

		err = CreateTable(ctx, &TestModel{})
		assert.NoError(t, err)

		record := &TestModel{Name: "John Doe", Email: "john.doe@example.com"}
		err = InsertData(ctx, record)
		assert.NoError(t, err)

		updates := map[string]interface{}{"name": "Jane Doe"}
		conditions := map[string]interface{}{"email": "john.doe@example.com"}
		err = UpdateData(ctx, &TestModel{}, updates, conditions)
		assert.NoError(t, err)

		var updatedRecords []TestModel
		err = FetchData(ctx, &updatedRecords, conditions)
		assert.NoError(t, err)
		assert.Len(t, updatedRecords, 1)
		assert.Equal(t, "Jane Doe", updatedRecords[0].Name)

		cleanDatabase(t, db)
	})

	t.Run("TestDeleteData", func(t *testing.T) {
		ctx := context.Background()
		_, err := Connect(config)
		assert.NoError(t, err)

		err = CreateTable(ctx, &TestModel{})
		assert.NoError(t, err)

		record := &TestModel{Name: "John Doe", Email: "john.doe@example.com"}
		err = InsertData(ctx, record)
		assert.NoError(t, err)

		conditions := map[string]interface{}{"email": "john.doe@example.com"}
		err = DeleteData(ctx, &TestModel{}, conditions)
		assert.NoError(t, err)

		var deletedRecords []TestModel
		err = FetchData(ctx, &deletedRecords, conditions)
		assert.NoError(t, err)
		assert.Len(t, deletedRecords, 0)

		cleanDatabase(t, db)
	})

	t.Run("TestExecuteCustomQuery", func(t *testing.T) {
		ctx := context.Background()
		_, err := Connect(config)
		assert.NoError(t, err)

		err = ExecuteCustomQuery(ctx, "CREATE TABLE test_table_custom_query (id SERIAL PRIMARY KEY, name VARCHAR(100))")
		assert.NoError(t, err)
		assert.True(t, IsTableExists(ctx, "test_table_custom_query"))

		cleanDatabase(t, db)
	})

	t.Run("TestIsTableExists", func(t *testing.T) {
		ctx := context.Background()
		_, err := Connect(config)
		assert.NoError(t, err)

		assert.False(t, IsTableExists(ctx, "non_existent_table"))

		err = CreateTable(ctx, &TestModel{}, "non_existent_table")
		assert.NoError(t, err)

		assert.True(t, IsTableExists(ctx, "non_existent_table"))

		cleanDatabase(t, db)
	})

	t.Run("TestQueryTimeout", func(t *testing.T) {
		ctx := context.Background()

		db, err := Connect(config)
		assert.NoError(t, err)
		assert.NotNil(t, db)

		err = ExecuteCustomQuery(ctx, "CREATE TABLE IF NOT EXISTS test_timeout (id SERIAL PRIMARY KEY, value TEXT)")
		assert.NoError(t, err)

		err = ExecuteCustomQuery(ctx, "INSERT INTO test_timeout (value) SELECT md5(random()::text) FROM generate_series(1, 100000)")
		assert.NoError(t, err)

		err = ExecuteCustomQuery(ctx, "SELECT pg_sleep(2), t1.* FROM test_timeout t1 CROSS JOIN test_timeout t2")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "canceling statement due to statement timeout")
		cleanDatabase(t, db)
	})

	t.Run("TestQueryTimeoutWithContext", func(t *testing.T) {
		// Create a context with a 2-second timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		db, err := Connect(config)
		assert.NoError(t, err)
		assert.NotNil(t, db)

		err = ExecuteCustomQuery(ctx, "CREATE TABLE IF NOT EXISTS test_timeout (id SERIAL PRIMARY KEY, value TEXT)")
		assert.NoError(t, err)

		err = ExecuteCustomQuery(ctx, "INSERT INTO test_timeout (value) SELECT md5(random()::text) FROM generate_series(1, 100000)")
		assert.NoError(t, err)

		err = ExecuteCustomQuery(ctx, "SELECT pg_sleep(2), t1.* FROM test_timeout t1 CROSS JOIN test_timeout t2")

		// Check if the error is a context deadline exceeded error
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout: context deadline exceeded")

		cleanDatabase(t, db)
	})

}
