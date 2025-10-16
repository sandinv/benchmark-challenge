package database

import (
	"context"
	"time"
)

const queryTimeout = 3 * time.Second
const query = `
    SELECT 
        time_bucket('1 minute', ts) AS bucket,
        MAX(usage) AS max_usage,
        MIN(usage) AS min_usage
     FROM cpu_usage
     WHERE host = $1 AND ts >= $2 AND ts <= $3
     GROUP BY bucket
     ORDER BY bucket`

// QueryParams represents parameters for a CPU usage query
type QueryParams struct {
	Hostname  string
	StartTime time.Time
	EndTime   time.Time
}

// Execute runs a query with the given parameters
func (d *Database) Execute(params QueryParams) error {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	rows, err := d.db.QueryContext(ctx, query, params.Hostname, params.StartTime, params.EndTime)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Consume all rows - each row represents one minute with max/min CPU usage
	for rows.Next() {
		var (
			bucket   time.Time
			maxUsage float64
			minUsage float64
		)
		if err := rows.Scan(&bucket, &maxUsage, &minUsage); err != nil {
			return err
		}
		// Data is not stored since we are only interested in the benchmark of the queries
	}

	return rows.Err()
}
