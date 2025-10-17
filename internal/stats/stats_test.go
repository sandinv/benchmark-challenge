package stats

import (
	"testing"
	"time"
)

func TestRecord(t *testing.T) {
	s := New()

	durations := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		150 * time.Millisecond,
	}

	for _, d := range durations {
		s.Record(d)
	}

	if s.TotalQueries != 3 {
		t.Errorf("Expected TotalQueries to be 3, got %d", s.TotalQueries)
	}
	if len(s.durations) != 3 {
		t.Errorf("Expected 3 durations, got %d", len(s.durations))
	}
}

func TestRecordError(t *testing.T) {
	s := New()

	s.Record(100 * time.Millisecond)
	s.RecordError()
	s.RecordError()

	if s.TotalQueries != 3 {
		t.Errorf("Expected TotalQueries to be 3, got %d", s.TotalQueries)
	}
	if len(s.durations) != 1 {
		t.Errorf("Expected 1 duration (errors don't add durations), got %d", len(s.durations))
	}
}

func TestCompute(t *testing.T) {
	s := New()

	durations := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		150 * time.Millisecond,
		300 * time.Millisecond,
		250 * time.Millisecond,
	}

	for _, d := range durations {
		s.Record(d)
	}

	s.Compute()

	// Test Min
	if s.MinTime != 100*time.Millisecond {
		t.Errorf("Expected MinTime to be 100ms, got %v", s.MinTime)
	}

	// Test Max
	if s.MaxTime != 300*time.Millisecond {
		t.Errorf("Expected MaxTime to be 300ms, got %v", s.MaxTime)
	}

	// Test Average (100+200+150+300+250)/5 = 200
	expectedAvg := 200 * time.Millisecond
	if s.AvgTime != expectedAvg {
		t.Errorf("Expected AvgTime to be %v, got %v", expectedAvg, s.AvgTime)
	}

	// Test Median (sorted: 100, 150, 200, 250, 300) -> 200
	if s.MedianTime != 200*time.Millisecond {
		t.Errorf("Expected MedianTime to be 200ms, got %v", s.MedianTime)
	}
}

func TestComputeEvenCount(t *testing.T) {
	s := New()

	durations := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
		400 * time.Millisecond,
	}

	for _, d := range durations {
		s.Record(d)
	}

	s.Compute()

	// Median of even count: (200 + 300) / 2 = 250
	expectedMedian := 250 * time.Millisecond
	if s.MedianTime != expectedMedian {
		t.Errorf("Expected MedianTime to be %v, got %v", expectedMedian, s.MedianTime)
	}
}

func TestPercentiles(t *testing.T) {
	s := New()

	// Add 100 durations from 1ms to 100ms
	for i := 1; i <= 100; i++ {
		s.Record(time.Duration(i) * time.Millisecond)
	}

	s.Compute()

	// P90 should be around 90ms
	if s.P90 < 89*time.Millisecond || s.P90 > 91*time.Millisecond {
		t.Errorf("Expected P90 to be around 90ms, got %v", s.P90)
	}

	// P95 should be around 95ms
	if s.P95 < 94*time.Millisecond || s.P95 > 96*time.Millisecond {
		t.Errorf("Expected P95 to be around 95ms, got %v", s.P95)
	}

	// P99 should be around 99ms
	if s.P99 < 98*time.Millisecond || s.P99 > 100*time.Millisecond {
		t.Errorf("Expected P99 to be around 99ms, got %v", s.P99)
	}
}

func TestComputeEmpty(t *testing.T) {
	s := New()
	s.Compute() // Should not panic

	if s.MinTime != 0 {
		t.Errorf("Expected MinTime to be 0 for empty stats, got %v", s.MinTime)
	}
}

func TestConcurrentRecords(t *testing.T) {
	s := New()

	// Concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				s.Record(time.Millisecond)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	if s.TotalQueries != 1000 {
		t.Errorf("Expected 1000 queries, got %d", s.TotalQueries)
	}
	if len(s.durations) != 1000 {
		t.Errorf("Expected 1000 durations, got %d", len(s.durations))
	}
}
