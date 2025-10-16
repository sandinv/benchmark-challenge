// Package database provides functions for connecting to a PostgreSQL-compatible database,
// configuring the connection pool, and executing queries for the benchmarking tool.
//
// It handles validation of connection strings, pinging the database to ensure availability,
// and setting up optimal connection pool settings based on worker count.
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

	maxOpenConns := workers
	if workers < 5 {
		maxOpenConns = maxOpenConns * 2
	}

	// Max connection would equal to workers * 2 if the number of workers < 5
	d.db.SetMaxOpenConns(maxOpenConns)
	d.db.SetMaxIdleConns(workers)
	d.db.SetConnMaxLifetime(5 * time.Minute)
}
