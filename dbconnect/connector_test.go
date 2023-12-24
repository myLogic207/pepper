package dbconnect

import (
	"context"
	"testing"

	"github.com/myLogic207/gotils/config"
	"github.com/stretchr/testify/assert"
)

var dbTestConfig = map[string]interface{}{
	"TYPE": PostgresDBType,
	// Add other required configuration values
	"HOST":     "localhost",
	"PORT":     "54321",
	"USERNAME": "postgres",
	"PASSWORD": "postgretest",
	"NAME":     "postgres",
}

func TestNewDB_Postgres(t *testing.T) {
	ctx := context.Background()
	conf, err := config.WithInitialValues(ctx, dbTestConfig)
	if err != nil {
		t.Fatal(err)
	}

	db := &DB{
		conf: conf,
	}

	if err := db.Connect(ctx, nil); err != nil {
		t.Fatal(err)
	}

	// Check if the database connection is open
	err = db.Ping()
	assert.NoError(t, err)

	// Close the database connection
	err = db.Close(context.Background())
	assert.NoError(t, err)
}

// Similar tests can be added for MySQL and MSSQL

func TestCheckTableExists(t *testing.T) {
	ctx := context.Background()
	conf, err := config.WithInitialValues(ctx, dbTestConfig)
	assert.NoError(t, err)
	db := &DB{}
	if err := db.Connect(ctx, conf); err != nil {
		t.Fatal(err)
	}

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
