package dbconnect_test

import (
	"context"
	"testing"

	"github.com/myLogic207/gotils/config"
	"github.com/myLogic207/pepper/dbconnect"
	"github.com/stretchr/testify/assert"
)

func TestNewDB_Postgres(t *testing.T) {
	conf := config.NewWithInitialValues(map[string]interface{}{
		"TYPE": dbconnect.PostgresDBType,
		// Add other required configuration values
	})
	db, err := dbconnect.New(conf)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	// Check if the database connection is open
	err = db.Ping()
	assert.NoError(t, err)

	// Close the database connection
	err = db.Close(context.Background())
	assert.NoError(t, err)
}

// Similar tests can be added for MySQL and MSSQL

func TestCheckTableExists(t *testing.T) {
	conf := config.NewWithInitialValues(map[string]interface{}{
		"TYPE": dbconnect.PostgresDBType,
		// Add other required configuration values
	})
	db, err := dbconnect.New(conf)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	// Use a test table name
	tableName := "test_table"

	// Check if the table exists (should be false initially)
	exists, err := db.CheckTableExists(context.Background(), tableName)
	assert.NoError(t, err)
	assert.False(t, exists)

	// Create the test table
	_, err = db.ExecContext(context.Background(), "CREATE TABLE "+tableName+" (id SERIAL PRIMARY KEY, name VARCHAR(255))")
	assert.NoError(t, err)

	// Check if the table exists (should be true now)
	exists, err = db.CheckTableExists(context.Background(), tableName)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Clean up: drop the test table
	_, err = db.ExecContext(context.Background(), "DROP TABLE "+tableName)
	assert.NoError(t, err)

	// Close the database connection
	err = db.Close(context.Background())
	assert.NoError(t, err)
}
