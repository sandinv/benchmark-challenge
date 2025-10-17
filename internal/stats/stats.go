// Package stats provides tools for recording, tracking, and analyzing
// statistics related to query execution.
//
// It allows you to record query durations, count errors, and compute
// summary metrics such as total time, min/max/average durations, median,
// and percentiles (P90, P95, P99). This package is useful for
// benchmarking database queries or other time-sensitive operations.
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
	_, _ = fmt.Fprintln(out, "\n"+strings.Repeat("=", 60))
	_, _ = fmt.Fprintln(out, "BENCHMARK RESULTS")
	_, _ = fmt.Fprintln(out, strings.Repeat("=", 60))
	_, _ = fmt.Fprintf(out, "Number of queries processed: %d\n", s.TotalQueries)
	_, _ = fmt.Fprintf(out, "Total processing time:       %v\n", s.ProcessingTime)

	if len(s.durations) > 0 {
		_, _ = fmt.Fprintf(out, "Successful queries:          %d/%d (%.1f%%)\n\n",
			len(s.durations), s.TotalQueries,
			float64(len(s.durations))/float64(s.TotalQueries)*100)

		_, _ = fmt.Fprintln(out, "Query Time Statistics:")
		_, _ = fmt.Fprintf(out, "  Minimum:     %v\n", s.MinTime)
		_, _ = fmt.Fprintf(out, "  Average:     %v\n", s.AvgTime)
		_, _ = fmt.Fprintf(out, "  Median:      %v\n", s.MedianTime)
		_, _ = fmt.Fprintf(out, "  Maximum:     %v\n\n", s.MaxTime)

		_, _ = fmt.Fprintln(out, "Percentiles:")
		_, _ = fmt.Fprintf(out, "  P90:          %v\n", s.P90)
		_, _ = fmt.Fprintf(out, "  P95:          %v\n", s.P95)
		_, _ = fmt.Fprintf(out, "  P99:          %v\n", s.P99)
	} else {
		_, _ = fmt.Fprintln(out, "No successful queries to report timing statistics")
	}
	_, _ = fmt.Fprintln(out, strings.Repeat("=", 60))
}
