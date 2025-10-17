// Package benchmark orchestrates concurrent query execution with worker pools and result collection.
//
// It implements a producer-consumer pattern where:
//   - CSV parser distributes queries to worker-specific channels based on hostname affinity
//   - Workers execute queries concurrently and send results to a collector
//   - Result collector aggregates timing statistics
//
// The package supports graceful shutdown through context cancellation and strict mode
// for data validation. All workers respect context cancellation and will stop processing
// when the context is cancelled.
package benchmark

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/sandinv/benchmark/internal/database"
	"github.com/sandinv/benchmark/internal/parser"
	"github.com/sandinv/benchmark/internal/stats"
)

const workerChannelSize = 10

// Runner orchestrates the benchmark execution
type Runner struct {
	db         *database.Database
	workers    int
	strictMode bool
}

// NewRunner creates a new benchmark runner
func NewRunner(db *database.Database, workers int, strictMode bool) *Runner {
	db.ConfigurePool(workers)
	return &Runner{
		db:         db,
		workers:    workers,
		strictMode: strictMode,
	}
}

// Run executes the benchmark and returns statistics
func (r *Runner) Run(ctx context.Context, input io.Reader) (*stats.Statistics, error) {

	statistics := stats.New()
	startTime := time.Now()

	// Create worker-specific channels (one per worker for hostname affinity)
	workerChannels := make([]chan database.QueryParams, r.workers)
	for i := 0; i < r.workers; i++ {
		workerChannels[i] = make(chan database.QueryParams, workerChannelSize)
	}

	// Create results channel
	results := make(chan result, workerChannelSize)

	// Start workers
	var workerWg sync.WaitGroup
	for i := 0; i < r.workers; i++ {
		workerWg.Go(func() {
			r.worker(ctx, workerChannels[i], results)
		})
	}

	// Start result collector
	var collectorWg sync.WaitGroup
	collectorWg.Go(func() {
		r.collectResults(results, statistics)
	})

	// Parse CSV and distribute queries to workers based on hostname
	csvParser := parser.NewCSVParser(input, r.strictMode)
	if err := csvParser.ParseAndDistribute(ctx, workerChannels); err != nil {
		if r.strictMode {
			// In strict mode, return the error immediately
			return nil, fmt.Errorf("CSV parsing error: %w", err)
		}
		log.Printf("Error parsing CSV: %v", err)
	}

	// Close all worker channels and wait for workers
	for i := 0; i < r.workers; i++ {
		close(workerChannels[i])
	}
	workerWg.Wait()

	// Close results channel and wait for collector
	close(results)
	collectorWg.Wait()

	// Finalize statistics by calculating the processing time and the stats
	statistics.ProcessingTime = time.Since(startTime)
	statistics.Compute()

	return statistics, nil
}

// result represents the outcome of a single query execution
type result struct {
	Duration time.Duration
	Error    error
}

// worker processes queries from the channel
func (r *Runner) worker(ctx context.Context, queries <-chan database.QueryParams, results chan<- result) {
	for {
		select {
		case <-ctx.Done():
			// Context cancelled, exit gracefully
			return
		case params, ok := <-queries:
			if !ok {
				// Channel closed, exit gracefully
				return
			}

			start := time.Now()
			err := r.db.Execute(ctx, params)
			duration := time.Since(start)

			// Try to send result, but respect context cancellation
			select {
			case <-ctx.Done():
				return
			case results <- result{
				Duration: duration,
				Error:    err,
			}:
			}
		}
	}
}

// collectResults aggregates query results
func (r *Runner) collectResults(results <-chan result, statistics *stats.Statistics) {
	for res := range results {
		if res.Error != nil {
			log.Printf("Query error: %v", res.Error)
			statistics.RecordError()
		} else {
			statistics.Record(res.Duration)
		}
	}
}
