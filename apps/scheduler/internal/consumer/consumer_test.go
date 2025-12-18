package consumer

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hhace/taskflow/apps/scheduler/config"
	"gorm.io/driver/postgres"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupMockDB creates a mock database connection using sqlmock
func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	require.NoError(t, err)

	return gormDB, mock
}

// buildConsumer creates a ResultConsumer with only the DB needed for unit tests.
func buildConsumer(db *gorm.DB) *ResultConsumer {
	return &ResultConsumer{
		config:   &config.Config{}, // not needed for handleResult/update/save
		natsConn: nil,              // not required for these tests
		db:       db,
		sub:      nil,
	}
}