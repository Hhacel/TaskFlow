package persistence

import (
	"os"
	"testing"
	"time"

	"github.com/hhace/taskflow/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := getTestDBConfig()

	t.Run("successful connection", func(t *testing.T) {
		db, err := Connect(config, 3)
		require.NoError(t, err)
		require.NotNil(t, db)
		defer Close(db)

		// Verify connection is active
		sqlDB, err := db.DB()
		require.NoError(t, err)
		err = sqlDB.Ping()
		assert.NoError(t, err)
	})

	t.Run("connection with retry on failure", func(t *testing.T) {
		badConfig := config
		badConfig.Port = "9999" // Invalid port

		db, err := Connect(badConfig, 2)
		assert.Error(t, err)
		assert.Nil(t, db)
		assert.Contains(t, err.Error(), "failed to connect to database after")
	})

	t.Run("connection pool configuration", func(t *testing.T) {
		db, err := Connect(config, 3)
		require.NoError(t, err)
		require.NotNil(t, db)
		defer Close(db)

		sqlDB, err := db.DB()
		require.NoError(t, err)

		stats := sqlDB.Stats()
		assert.Equal(t, config.MaxOpenConns, stats.MaxOpenConnections)
	})
}

func TestClose(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := getTestDBConfig()

	t.Run("close valid database", func(t *testing.T) {
		db, err := Connect(config, 3)
		require.NoError(t, err)

		err = Close(db)
		assert.NoError(t, err)

		// Verify connection is closed
		sqlDB, err := db.DB()
		require.NoError(t, err)
		err = sqlDB.Ping()
		assert.Error(t, err) // Should error because connection is closed
	})

	t.Run("close nil database", func(t *testing.T) {
		err := Close(nil)
		assert.NoError(t, err)
	})
}

func TestMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := getTestDBConfig()

	t.Run("migrate with nil database", func(t *testing.T) {
		err := Migrate(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database not connected")
	})

	t.Run("successful migration", func(t *testing.T) {
		db, err := Connect(config, 3)
		require.NoError(t, err)
		defer Close(db)

		err = Migrate(db)
		assert.NoError(t, err)

		// Verify tables were created
		var tableExists bool
		err = db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'tasks')").Scan(&tableExists).Error
		assert.NoError(t, err)
		assert.True(t, tableExists)

		err = db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'task_execution_results')").Scan(&tableExists).Error
		assert.NoError(t, err)
		assert.True(t, tableExists)
	})

	t.Run("idempotent migration", func(t *testing.T) {
		db, err := Connect(config, 3)
		require.NoError(t, err)
		defer Close(db)

		// Run migration twice
		err = Migrate(db)
		assert.NoError(t, err)

		err = Migrate(db)
		assert.NoError(t, err) // Should not error on second run
	})
}

func TestHealth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := getTestDBConfig()

	t.Run("health check on valid database", func(t *testing.T) {
		db, err := Connect(config, 3)
		require.NoError(t, err)
		defer Close(db)

		err = Health(db)
		assert.NoError(t, err)
	})

	t.Run("health check on nil database", func(t *testing.T) {
		err := Health(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database not connected")
	})

	t.Run("health check on closed database", func(t *testing.T) {
		db, err := Connect(config, 3)
		require.NoError(t, err)

		Close(db)

		err = Health(db)
		assert.Error(t, err) // Should error because connection is closed
	})
}

func TestFullDatabaseWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := getTestDBConfig()

	t.Run("connect, migrate, use, close", func(t *testing.T) {
		// Connect
		db, err := Connect(config, 3)
		require.NoError(t, err)
		require.NotNil(t, db)

		// Migrate
		err = Migrate(db)
		require.NoError(t, err)

		// Use database - create a test taskObj
		taskObj := &task.Task{
			Schedule: "*/5 * * * *",
			Command:  task.StringArray{"echo", "test"},
			Status:   task.TaskStatusCreated,
		}
		err = db.Create(taskObj).Error
		require.NoError(t, err)
		assert.NotEqual(t, "", taskObj.ID.String())

		// Verify task was created
		var retrievedTask task.Task
		err = db.First(&retrievedTask, "id = ?", taskObj.ID).Error
		require.NoError(t, err)
		assert.Equal(t, taskObj.Schedule, retrievedTask.Schedule)
		assert.Equal(t, taskObj.Command, retrievedTask.Command)

		// Clean up - delete test task
		err = db.Delete(&task.Task{}, "id = ?", taskObj.ID).Error
		require.NoError(t, err)

		// Health check
		err = Health(db)
		assert.NoError(t, err)

		// Close
		err = Close(db)
		assert.NoError(t, err)
	})
}

func getTestDBConfig() Config {
	return Config{
		Host:                   getEnv("TEST_DB_HOST", "localhost"),
		Port:                   getEnv("TEST_DB_PORT", "5432"),
		User:                   getEnv("TEST_DB_USER", "taskflow"),
		Password:               getEnv("TEST_DB_PASSWORD", "taskflow"),
		Database:               getEnv("TEST_DB_NAME", "taskflow"),
		SSLMode:                getEnv("TEST_DB_SSLMODE", "disable"),
		MaxOpenConns:           5,
		MaxIdleConns:           2,
		ConnMaxLifetimeMinutes: 15,
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func TestConnectionPoolSettings(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := getTestDBConfig()
	config.MaxOpenConns = 10
	config.MaxIdleConns = 3
	config.ConnMaxLifetimeMinutes = 30

	db, err := Connect(config, 3)
	require.NoError(t, err)
	defer Close(db)

	sqlDB, err := db.DB()
	require.NoError(t, err)

	// Verify pool settings
	stats := sqlDB.Stats()
	assert.Equal(t, config.MaxOpenConns, stats.MaxOpenConnections)

	// Note: MaxIdleConns and ConnMaxLifetime can't be directly verified from stats
	// but we can verify the connection works with these settings
	err = sqlDB.Ping()
	assert.NoError(t, err)
}

func TestConcurrentConnections(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := getTestDBConfig()
	config.MaxOpenConns = 5

	db, err := Connect(config, 3)
	require.NoError(t, err)
	defer Close(db)

	// Run multiple queries concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			var result int
			err := db.Raw("SELECT 1").Scan(&result).Error
			assert.NoError(t, err)
			assert.Equal(t, 1, result)
			done <- true
		}()
	}

	// Wait for all goroutines with timeout
	timeout := time.After(5 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Success
		case <-timeout:
			t.Fatal("Timeout waiting for concurrent queries")
		}
	}
}

func TestUUIDExtension(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := getTestDBConfig()
	db, err := Connect(config, 3)
	require.NoError(t, err)
	defer Close(db)

	err = Migrate(db)
	require.NoError(t, err)

	// Verify uuid-ossp extension exists
	var extensionExists bool
	err = db.Raw("SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'uuid-ossp')").Scan(&extensionExists).Error
	assert.NoError(t, err)
	assert.True(t, extensionExists)
}
