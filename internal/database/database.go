// Package database implements utility functions to access the database
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	pq "github.com/lib/pq"
)

type Database struct {
	db *sql.DB
}

// Connect establishes a connection to the database using connection string provided and verifies that it is connected
func Connect(connectionString string) (*Database, error) {

	// Validate if the connection string is valid
	_, err := pq.ParseURL(connectionString)
	if err != nil {
		return nil, fmt.Errorf("invalid database connection string: %w", err)
	}

	db, err := sql.Open("postgres", connectionString)

	if err != nil {
		return nil, err
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Once the database connection pool is created, we verify that we can connect using 1 second timeout
	if err := db.PingContext(timeoutCtx); err != nil {
		return nil, err
	}

	return &Database{db}, nil
}

// ConfigurePool sets up the connection pool for optimal performance
func (d *Database) ConfigurePool(workers int) {
	d.db.SetMaxOpenConns(workers * 2)
	d.db.SetMaxIdleConns(workers)
	d.db.SetConnMaxLifetime(time.Minute)
}
