package parser

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sandinv/benchmark/internal/database"
)

func TestHostnameHash(t *testing.T) {
	tests := []struct {
		hostname string
		want     bool // Should be non-negative
	}{
		{"host_000001", true},
		{"host_000002", true},
		{"server123", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			hash := hostnameHash(tt.hostname)
			if hash < 0 {
				t.Errorf("hostnameHash(%q) returned negative value: %d", tt.hostname, hash)
			}
		})
	}
}

func TestHostnameHashConsistency(t *testing.T) {
	hostname := "host_000001"
	hash1 := hostnameHash(hostname)
	hash2 := hostnameHash(hostname)

	if hash1 != hash2 {
		t.Errorf("hostnameHash is not consistent: %d != %d", hash1, hash2)
	}
}

func TestHostnameHashDistribution(t *testing.T) {
	// Same hostname should always go to the same worker
	hostname := "host_000001"
	numWorkers := 5

	worker1 := hostnameHash(hostname) % numWorkers
	worker2 := hostnameHash(hostname) % numWorkers

	if worker1 != worker2 {
		t.Errorf("Same hostname mapped to different workers: %d != %d", worker1, worker2)
	}
}

func TestParseRecord(t *testing.T) {
	tests := []struct {
		name    string
		record  []string
		wantErr bool
	}{
		{
			name:    "valid record",
			record:  []string{"host_000001", "2017-01-01 08:59:22", "2017-01-01 09:59:22"},
			wantErr: false,
		},
		{
			name:    "invalid field count",
			record:  []string{"host_000001", "2017-01-01 08:59:22"},
			wantErr: true,
		},
		{
			name:    "invalid start time",
			record:  []string{"host_000001", "invalid-date", "2017-01-01 09:59:22"},
			wantErr: true,
		},
		{
			name:    "invalid end time",
			record:  []string{"host_000001", "2017-01-01 08:59:22", "invalid-date"},
			wantErr: true,
		},
		{
			name:    "empty hostname",
			record:  []string{"", "2017-01-01 08:59:22", "2017-01-01 09:59:22"},
			wantErr: false, // Empty hostname is allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewCSVParser(nil, false)
			params, err := parser.parseRecord(tt.record)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseRecord() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if params.Hostname != tt.record[0] {
					t.Errorf("Expected hostname %q, got %q", tt.record[0], params.Hostname)
				}
			}
		})
	}
}

func TestParseRecordTimes(t *testing.T) {
	parser := NewCSVParser(nil, false)
	record := []string{"host_000001", "2017-01-01 08:59:22", "2017-01-01 09:59:22"}

	params, err := parser.parseRecord(record)
	if err != nil {
		t.Fatalf("parseRecord() failed: %v", err)
	}

	expectedStart, _ := time.Parse("2006-01-02 15:04:05", "2017-01-01 08:59:22")
	expectedEnd, _ := time.Parse("2006-01-02 15:04:05", "2017-01-01 09:59:22")

	if !params.StartTime.Equal(expectedStart) {
		t.Errorf("Expected start time %v, got %v", expectedStart, params.StartTime)
	}
	if !params.EndTime.Equal(expectedEnd) {
		t.Errorf("Expected end time %v, got %v", expectedEnd, params.EndTime)
	}
}

func TestParseAndDistributeValidCSV(t *testing.T) {
	csvData := `hostname,start_time,end_time
host_000001,2017-01-01 08:59:22,2017-01-01 09:59:22
host_000002,2017-01-01 10:00:00,2017-01-01 11:00:00
host_000003,2017-01-01 12:00:00,2017-01-01 13:00:00`

	reader := strings.NewReader(csvData)
	parser := NewCSVParser(reader, false)

	numWorkers := 3
	workerChannels := make([]chan database.QueryParams, numWorkers)
	for i := 0; i < numWorkers; i++ {
		workerChannels[i] = make(chan database.QueryParams, 10)
	}

	ctx := context.Background()

	// Parse in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- parser.ParseAndDistribute(ctx, workerChannels)
		for i := 0; i < numWorkers; i++ {
			close(workerChannels[i])
		}
	}()

	// Collect results
	totalReceived := 0
	for i := 0; i < numWorkers; i++ {
		for range workerChannels[i] {
			totalReceived++
		}
	}

	err := <-errChan
	if err != nil {
		t.Errorf("ParseAndDistribute failed: %v", err)
	}

	if totalReceived != 3 {
		t.Errorf("Expected 3 queries, got %d", totalReceived)
	}
}

func TestParseAndDistributeStrictMode(t *testing.T) {
	// CSV with invalid record
	csvData := `hostname,start_time,end_time
host_000001,2017-01-01 08:59:22,2017-01-01 09:59:22
host_000002,invalid-date,2017-01-01 11:00:00
host_000003,2017-01-01 12:00:00,2017-01-01 13:00:00`

	reader := strings.NewReader(csvData)
	parser := NewCSVParser(reader, true) // Strict mode enabled

	numWorkers := 2
	workerChannels := make([]chan database.QueryParams, numWorkers)
	for i := 0; i < numWorkers; i++ {
		workerChannels[i] = make(chan database.QueryParams, 10)
	}

	ctx := context.Background()

	err := parser.ParseAndDistribute(ctx, workerChannels)

	// Should return error in strict mode
	if err == nil {
		t.Error("Expected error in strict mode with invalid record, got nil")
	}
}

func TestParseAndDistributeLenientMode(t *testing.T) {
	// CSV with invalid record
	csvData := `hostname,start_time,end_time
host_000001,2017-01-01 08:59:22,2017-01-01 09:59:22
host_000002,invalid-date,2017-01-01 11:00:00
host_000003,2017-01-01 12:00:00,2017-01-01 13:00:00`

	reader := strings.NewReader(csvData)
	parser := NewCSVParser(reader, false) // Lenient mode

	numWorkers := 2
	workerChannels := make([]chan database.QueryParams, numWorkers)
	for i := 0; i < numWorkers; i++ {
		workerChannels[i] = make(chan database.QueryParams, 10)
	}

	ctx := context.Background()

	// Parse in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- parser.ParseAndDistribute(ctx, workerChannels)
		for i := 0; i < numWorkers; i++ {
			close(workerChannels[i])
		}
	}()

	// Collect results
	totalReceived := 0
	for i := 0; i < numWorkers; i++ {
		for range workerChannels[i] {
			totalReceived++
		}
	}

	err := <-errChan
	// Should not return error in lenient mode
	if err != nil {
		t.Errorf("Lenient mode should not error, got: %v", err)
	}

	// Should receive 2 valid records (skipping the invalid one)
	if totalReceived != 2 {
		t.Errorf("Expected 2 valid queries, got %d", totalReceived)
	}
}

func TestParseAndDistributeContextCancellation(t *testing.T) {
	csvData := `hostname,start_time,end_time
host_000001,2017-01-01 08:59:22,2017-01-01 09:59:22
host_000002,2017-01-01 10:00:00,2017-01-01 11:00:00`

	reader := strings.NewReader(csvData)
	parser := NewCSVParser(reader, false)

	numWorkers := 2
	workerChannels := make([]chan database.QueryParams, numWorkers)
	for i := 0; i < numWorkers; i++ {
		workerChannels[i] = make(chan database.QueryParams, 10)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := parser.ParseAndDistribute(ctx, workerChannels)

	// Should return context error
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

func TestParseAndDistributeEmptyCSV(t *testing.T) {
	csvData := `hostname,start_time,end_time`

	reader := strings.NewReader(csvData)
	parser := NewCSVParser(reader, false)

	numWorkers := 2
	workerChannels := make([]chan database.QueryParams, numWorkers)
	for i := 0; i < numWorkers; i++ {
		workerChannels[i] = make(chan database.QueryParams, 10)
	}

	ctx := context.Background()

	// Parse in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- parser.ParseAndDistribute(ctx, workerChannels)
		for i := 0; i < numWorkers; i++ {
			close(workerChannels[i])
		}
	}()

	// Collect results
	totalReceived := 0
	for i := 0; i < numWorkers; i++ {
		for range workerChannels[i] {
			totalReceived++
		}
	}

	err := <-errChan
	if err != nil {
		t.Errorf("ParseAndDistribute failed on empty CSV: %v", err)
	}

	if totalReceived != 0 {
		t.Errorf("Expected 0 queries from empty CSV, got %d", totalReceived)
	}
}
