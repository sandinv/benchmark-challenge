// Package parser provides functionality for parsing benchmark input records
// from CSV file or standard input, and distributing them to worker goroutines for concurrent query execution.
//
// The parser implements hostname-based affinity routing using FNV-1a hashing to ensure
// queries for the same hostname are consistently assigned to the same worker.
//
// It supports:
//   - Streaming CSV processing (line-by-line reading)
//   - Context cancellation for graceful shutdown
//   - Strict mode: exits immediately on any CSV reading or parsing error
//   - Lenient mode (default): logs errors and continues processing
package parser

import (
	"context"
	"encoding/csv"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"time"

	"github.com/sandinv/benchmark/internal/database"
)

// CSVParser parses CSV input and extracts query parameters
type CSVParser struct {
	reader     *csv.Reader
	strictMode bool
}

// NewCSVParser creates a new CSV parser
func NewCSVParser(input io.Reader, strictMode bool) *CSVParser {
	return &CSVParser{
		reader:     csv.NewReader(input),
		strictMode: strictMode,
	}
}

// ParseAndDistribute reads CSV input and distributes queries to workers based on hostname
// The records are read line by line to process large files with minimum impact
func (p *CSVParser) ParseAndDistribute(ctx context.Context, workerChannels []chan database.QueryParams) error {

	// Read and skip header
	if _, err := p.reader.Read(); err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	numWorkers := len(workerChannels)

	// Read and process records
	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		record, err := p.reader.Read()

		if err == io.EOF {
			break
		}

		// Handle errors on reading records
		if err != nil {
			if p.strictMode {
				return fmt.Errorf("error reading CSV record: %w", err)
			}
			log.Printf("Error reading CSV record: %v", err)
			continue
		}

		params, err := p.parseRecord(record)
		// Handle malformed records
		if err != nil {
			if p.strictMode {
				return fmt.Errorf("error parsing record: %w", err)
			}
			log.Printf("Error parsing record: %v", err)
			continue
		}

		// Assign to worker based on hostname hash
		// This ensures the same hostname always goes to the same worker
		workerID := hostnameHash(params.Hostname) % numWorkers

		// Try to send to worker channel, but respect context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case workerChannels[workerID] <- params:
		}
	}

	return nil
}

// hostnameHash returns a hash of the hostname for worker assignment
func hostnameHash(hostname string) int {
	h := fnv.New32a() // FNV-1a is fast and has good distribution
	h.Write([]byte(hostname))
	return int(h.Sum32() & 0x7FFFFFFF) // ensure non-negative int32
}

// parseRecord converts a CSV record to QueryParams
func (p *CSVParser) parseRecord(record []string) (database.QueryParams, error) {

	// Records that do have more or less than 3 fields are skipped and logged
	if len(record) != 3 {
		return database.QueryParams{}, fmt.Errorf("invalid record: expected 3 fields, got %d", len(record))
	}

	startTime, err := time.Parse("2006-01-02 15:04:05", record[1])
	if err != nil {
		return database.QueryParams{}, fmt.Errorf("invalid start time: %w", err)
	}

	endTime, err := time.Parse("2006-01-02 15:04:05", record[2])
	if err != nil {
		return database.QueryParams{}, fmt.Errorf("invalid end time: %w", err)
	}

	return database.QueryParams{
		Hostname:  record[0],
		StartTime: startTime,
		EndTime:   endTime,
	}, nil
}
