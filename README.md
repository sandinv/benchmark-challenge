# TimescaleDB Query Benchmark Tool

A command-line tool for benchmarking query performance against TimescaleDB with concurrent workers.

## Features

- **Concurrent Query Execution**: Configure multiple workers to execute queries in parallel
- **Streaming Input Processing**: Processes queries as they are read, without waiting for all input
- **Flexible Input**: Accepts CSV files or stdin
- **Comprehensive Statistics**: Reports query count, total processing time, and query duration statistics, including minimum, median, average, maximum, and percentile values.


## Prerequisites

- Go 1.25 or higher
- TimescaleDB instance running and accessible
- Database initialized with the schema from `cpu_usage.sql`

## Installation

1. Clone or download this repository.

2. Build the tool:
   ```bash
   make build
   ```
Alternatively, you can use the included docker-compose.yml file to spin up:
-	a PostgreSQL database with the appropriate schema and sample data
-	the CLI tool, ready to run benchmarks against it

## Usage

### Basic Usage

```bash
# Set the database connection string
export DATABASE_URL='postgres://postgres:password@localhost:5432/homework?sslmode=disable'

# Using a CSV file
./benchmark -inputFile query_params.csv -workers 4

# Using stdin
cat query_params.csv | ./benchmark -workers 4
```

A sample `query_params.csv` is included in this repository for testing purposes.

### Configuration

#### Database Connection

The tool uses the `DATABASE_URL` environment variable for database connection:

```bash
export DATABASE_URL='postgresql://user:password@host:port/database?sslmode=disable'
```

> **Note on SSL Support**: This tool currently does not support SSL/TLS connections. All connection strings must use `sslmode=disable`. This is suitable for local development and trusted networks, but not recommended for production environments with untrusted networks.

**Why environment variables?**
- Prevents password exposure in shell history
- Prevents password exposure in process lists
- Follows 12-factor app best practices
- Better security for production environments

You can create a `.env` file for local development:
```bash
# Create .env file
cat > .env << 'EOF'
DATABASE_URL='postgres://postgres:password@localhost:5432/homework?sslmode=disable'
POSTGRES_PW=password
EOF

# Load environment variables
source .env
```

#### Command-Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-workers` | 5 | Number of concurrent workers (should be equal or greater than 1) |
| `-inputFile` | "" | CSV file path (if empty, reads from stdin) |
| `-strict` | false | Strict mode: exit on any CSV reading or parsing error |

### CSV Format

The input CSV file should have the following format:

```csv
hostname,start_time,end_time
host_000008,2017-01-01 08:59:22,2017-01-01 09:59:22
host_000001,2017-01-02 13:02:02,2017-01-02 14:02:02
```

**Columns:**
- `hostname`: Host identifier to query
- `start_time`: Start timestamp (format: `YYYY-MM-DD HH:MM:SS`)
- `end_time`: End timestamp (format: `YYYY-MM-DD HH:MM:SS`)

### Examples

#### Example 1: Basic benchmark with 4 workers

```bash
export DATABASE_URL='postgres://postgres:password@localhost:5432/homework?sslmode=disable'
./benchmark -inputFile query_params.csv -workers 4
```

#### Example 2: Using .env file

```bash
# Create .env file
cat > .env << 'EOF'
DATABASE_URL='postgres://postgres:password@localhost:5432/homework?sslmode=disable'
POSTGRES_PW=password
EOF

# Load environment and run
source .env
./benchmark -inputFile query_params.csv -workers 8
```

#### Example 3: Piping from stdin

```bash
export DATABASE_URL='postgres://postgres:password@localhost:5432/homework?sslmode=disable'
cat query_params.csv | ./benchmark -workers 10
```

#### Example 4: Remote database

```bash
export DATABASE_URL='postgresql://admin:secret@db.example.com:5432/production?sslmode=disable'
./benchmark -inputFile query_params.csv -workers 4
```

#### Example 5: One-liner without exporting

```bash
DATABASE_URL='postgresql://postgres:password@localhost:5432/homework?sslmode=disable' \
  ./benchmark -inputFile query_params.csv -workers 4
```

#### Example 6: Strict mode for data validation

```bash
export DATABASE_URL='postgres://postgres:password@localhost:5432/homework?sslmode=disable'
# Exit immediately on any malformed CSV record
./benchmark -inputFile query_params.csv -workers 4 -strict
```

## Setting Up TimescaleDB

The Docker Compose setup automatically creates the database with schema and sample data.

1. Set up your environment:
   ```bash
   # Create environment file
   cat > .env << 'EOF'
   DATABASE_URL='postgres://postgres:password@localhost:5432/homework?sslmode=disable'
   POSTGRES_PW=password
   EOF
   source .env
   ```

2. Start TimescaleDB using Make:
   ```bash
   # The database image includes schema (cpu_usage.sql) and data (cpu_usage.csv)
   make db-up
   ```

3. (Optional) Check logs or access the database:
   ```bash
   # Check logs to see when initialization is complete
   docker-compose logs -f timescaledb
   
   # Or open a psql shell
   make db-shell
   ```

4. When finished, stop the database:
   ```bash
   make db-down
   ```

The TimescaleDB container will automatically:
- Create the `homework` database
- Enable TimescaleDB extension
- Create the `cpu_usage` hypertable
- Load all data from `cpu_usage.csv`

## Output

The tool outputs benchmark statistics including:

```
============================================================
BENCHMARK RESULTS
============================================================
Number of queries processed: 200
Total processing time:       90.052958ms
Successful queries:          200/200 (100.0%)

Query Time Statistics:
  Minimum:     843.875µs
  Average:     1.958101ms
  Median:      1.431354ms
  Maximum:     23.958666ms

Percentiles:
  P90:          2.078566ms
  P95:          2.284135ms
  P99:          19.64811ms
============================================================
```

## Performance Considerations

- **Worker Count**: More workers generally improve throughput, but too many can cause contention. Start with 4-8 workers and adjust based on your system.
- **Connection Pooling**: The tool automatically configures the connection pool based on worker count.
- **Large Files**: The tool streams input, so it can handle files larger than available memory.
- **Network Latency**: For remote databases, consider network latency when interpreting results.

## Architecture

The tool uses a producer-consumer pattern with hostname affinity:

1. **CSV Parser** (Producer): Reads CSV input and distributes query parameters to worker-specific channels
2. **Hostname-based Distribution**: Each hostname is consistently routed to the same worker using hash-based assignment based on FSV-2.
3. **Workers** (Consumers): Multiple goroutines, each reading from their own channel and executing queries
4. **Result Collector**: Aggregates timing data from all workers
5. **Statistics Calculator**: Computes final statistics after all queries complete

### Key Features

**Hostname Affinity**: Queries for the same hostname are always executed by the same worker. This is achieved by:
- Hashing the hostname to generate a consistent worker ID
- Each worker has its own dedicated channel
- The CSV parser routes queries to the appropriate worker channel based on the hash

**SQL Query**: Each query retrieves max and min CPU usage per minute for a given hostname and time range:
```sql
SELECT 
    time_bucket('1 minute', ts) AS bucket,
    MAX(usage) AS max_usage,
    MIN(usage) AS min_usage
FROM cpu_usage
WHERE host = $1 AND ts >= $2 AND ts <= $3
GROUP BY bucket
ORDER BY bucket
```

This design ensures:
- Streaming processing (no need to load entire file into memory)
- Concurrent query execution across multiple workers
- Same hostname always processed by same worker
- Efficient resource utilization
- Accurate timing measurements

## Troubleshooting

### Connection Refused
- Ensure TimescaleDB is running: `docker ps`
- Check the port mapping: `docker port <container_id>`

### Authentication Failed
- Verify password matches docker-compose.yml configuration
- Check user has permissions on the database

### Out of Memory
- Reduce the number of workers
- Check database connection pool settings

