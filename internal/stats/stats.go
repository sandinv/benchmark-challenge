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
	P50            time.Duration // 50th percentile (median)
	P90            time.Duration // 90th percentile
	P95            time.Duration // 95th percentile
	P99            time.Duration // 99th percentile

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

	// Sort durations for median and percentile calculations
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

	// Percentiles
	s.P90 = s.percentile(90)
	s.P95 = s.percentile(95)
	s.P99 = s.percentile(99)
}

// percentile calculates the given percentile from sorted durations
// Must be called with mutex locked and after durations are sorted
func (s *Statistics) percentile(p float64) time.Duration {
	if len(s.durations) == 0 {
		return 0
	}

	// Use linear interpolation method
	n := float64(len(s.durations))
	rank := (p / 100.0) * (n - 1)
	lower := int(rank)
	upper := lower + 1

	// Handle edge cases
	if upper >= len(s.durations) {
		return s.durations[len(s.durations)-1]
	}

	// Linear interpolation between the two nearest values
	fraction := rank - float64(lower)
	return time.Duration(float64(s.durations[lower]) +
		fraction*float64(s.durations[upper]-s.durations[lower]))
}

// Print outputs the statistics to the provided output
func (s *Statistics) Print(out io.Writer) {
	fmt.Fprintln(out, "\n"+strings.Repeat("=", 60))
	fmt.Fprintln(out, "BENCHMARK RESULTS")
	fmt.Fprintln(out, strings.Repeat("=", 60))
	fmt.Fprintf(out, "Number of queries processed: %d\n", s.TotalQueries)
	fmt.Fprintf(out, "Total processing time:       %v\n", s.ProcessingTime)

	if len(s.durations) > 0 {
		fmt.Fprintf(out, "Successful queries:          %d/%d (%.1f%%)\n\n",
			len(s.durations), s.TotalQueries,
			float64(len(s.durations))/float64(s.TotalQueries)*100)

		fmt.Fprintln(out, "Query Time Statistics:")
		fmt.Fprintf(out, "  Minimum:     %v\n", s.MinTime)
		fmt.Fprintf(out, "  Average:     %v\n", s.AvgTime)
		fmt.Fprintf(out, "  Median:      %v\n", s.MedianTime)
		fmt.Fprintf(out, "  Maximum:     %v\n\n", s.MaxTime)

		fmt.Fprintln(out, "Percentiles:")
		fmt.Fprintf(out, "  P90:          %v\n", s.P90)
		fmt.Fprintf(out, "  P95:          %v\n", s.P95)
		fmt.Fprintf(out, "  P99:          %v\n", s.P99)
	} else {
		fmt.Fprintln(out, "No successful queries to report timing statistics")
	}
	fmt.Fprintln(out, strings.Repeat("=", 60))
}
