// Package stats provides tools for recording, tracking, and analyzing
// statistics related to query execution.
//
// It allows you to record query durations, count errors, and compute
// summary metrics such as total time, min/max/average durations, and median.
// This package is useful for benchmarking database queries or other
// time-sensitive operations.
//
// Typical usage:
//
//	s := stats.New()
//	s.Record(duration)
//	s.RecordError()
//	...
//	s.Compute()
//	s.Print(os.Stdout)
//
// The package is safe for concurrent access.
package stats

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"
)

// Statistics holds benchmark statistics
type Statistics struct {
	TotalQueries   int
	ProcessingTime time.Duration
	MinTime        time.Duration
	MaxTime        time.Duration
	MedianTime     time.Duration
	AvgTime        time.Duration

	durations []time.Duration
	mu        sync.Mutex
}

// New creates a new Statistics instance
func New() *Statistics {
	return &Statistics{
		durations: make([]time.Duration, 0),
	}
}

// Record adds a query duration to the statistics
func (s *Statistics) Record(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalQueries++
	s.durations = append(s.durations, duration)
}

// RecordError increments the total query count for a failed query
func (s *Statistics) RecordError() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalQueries++
}

// Compute calculates the final statistics
func (s *Statistics) Compute() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.durations) == 0 {
		return
	}

	// Sort durations for median calculation
	slices.Sort(s.durations)

	// Min and Max
	s.MinTime = s.durations[0]
	s.MaxTime = s.durations[len(s.durations)-1]

	// Median
	mid := len(s.durations) / 2
	if len(s.durations)%2 == 0 {
		s.MedianTime = (s.durations[mid-1] + s.durations[mid]) / 2
	} else {
		s.MedianTime = s.durations[mid]
	}

	// Average
	var total time.Duration
	for _, d := range s.durations {
		total += d
	}
	s.AvgTime = total / time.Duration(len(s.durations))
}

// Print outputs the statistics to the provided output
func (s *Statistics) Print(out io.Writer) {
	fmt.Fprintln(out, "\n"+strings.Repeat("=", 60))
	fmt.Fprintln(out, "BENCHMARK RESULTS")
	fmt.Fprintln(out, strings.Repeat("=", 60))
	fmt.Fprintf(out, "Number of queries processed: %d\n", s.TotalQueries)
	fmt.Fprintf(out, "Total processing time:       %v\n", s.ProcessingTime)

	if len(s.durations) > 0 {
		fmt.Fprintf(out, "Minimum query time:          %v\n", s.MinTime)
		fmt.Fprintf(out, "Median query time:           %v\n", s.MedianTime)
		fmt.Fprintf(out, "Average query time:          %v\n", s.AvgTime)
		fmt.Fprintf(out, "Maximum query time:          %v\n", s.MaxTime)
		fmt.Fprintf(out, "Successful queries:          %d/%d (%.1f%%)\n",
			len(s.durations), s.TotalQueries,
			float64(len(s.durations))/float64(s.TotalQueries)*100)
	} else {
		fmt.Fprintln(out, "No successful queries to report timing statistics")
	}
	fmt.Fprintln(out, strings.Repeat("=", 60))
}
