package main

// TODO:
// Add to the README.md that SSL is not supported
// Add strict mode that would exit on any parsing/reading error
// Add a context propagation to handler graceful shutdown
import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"log"

	"github.com/sandinv/benchmark/internal/benchmark"
	"github.com/sandinv/benchmark/internal/database"
)

type Config struct {
	DatabaseConn string
	Workers      int
	InputFile    string
	StrictMode   bool
}

func init() {
	// Override default usage output
	flag.Usage = printUsage
}

func main() {

	config := parseFlags()

	if config.Workers <= 0 {
		log.Fatalf("workers should be equal or greater than 1")
	}

	parseConnectionString(&config)

	reader, closeFun, err := parseInputFile(config.InputFile)
	if err != nil {
		log.Fatalf("couldn't read input file: %s", err)
	}
	defer closeFun()

	db, err := database.Connect(config.DatabaseConn)
	if err != nil {
		log.Fatalf("can't establish a connection with the database %s", err)
	}

	// Setup context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupShutdown(cancel)

	runner := benchmark.NewRunner(db, config.Workers, config.StrictMode)
	stats, err := runner.Run(ctx, reader)
	if err != nil {
		log.Fatal(err)
	}

	stats.Print(os.Stdout)

}

func parseFlags() Config {

	config := Config{}

	flag.IntVar(&config.Workers, "workers", 5, "number of concurrent workers (should be equal or greater than 1)")
	flag.StringVar(&config.InputFile, "inputFile", "", "CSV file path ( if not provided, reads from stdin")
	flag.BoolVar(&config.StrictMode, "strict", false, "strict mode: exit on any CSV reading or parsing error (default: false)")

	flag.Parse()

	return config
}

func parseConnectionString(config *Config) {

	// Get connection string from environment variable
	config.DatabaseConn = os.Getenv("DATABASE_URL")

	if config.DatabaseConn == "" {
		// Provide a default for local development
		config.DatabaseConn = "postgres://postgres:password@localhost:5432/homework?sslmode=disable"
		fmt.Fprintf(os.Stderr, "Warning: DATABASE_URL not set, using default: %s\n", config.DatabaseConn)
	}

}

func parseInputFile(filepath string) (io.Reader, func(), error) {

	if filepath != "" {
		inputF, err := os.Open(filepath)
		if err != nil {
			return nil, nil, err
		}

		return inputF, func() {
			err = inputF.Close()
			if err != nil {
				log.Fatal(err)
			}
		}, nil
	}

	return os.Stdin, func() {}, nil

}

func setupShutdown(cancel func()) {
	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start a goroutine to handle shutdown signals
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)
		cancel()
	}()

}

// printUsage prints usage information
func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Database connection is configured via environment variable:\n")
	fmt.Fprintf(os.Stderr, "  DATABASE_URL - PostgreSQL connection string\n")
	fmt.Fprintf(os.Stderr, "  Example: postgres://user:password@localhost:5432/homework?sslmode=disable\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  export DATABASE_URL='postgres://postgres:password@localhost:5432/homework?sslmode=disable'\n")
	fmt.Fprintf(os.Stderr, "  %s -inputFile query_params.csv -workers 4\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  cat query_params.csv | %s -workers 4\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s -inputFile query_params.csv -workers 10 -strict\n", os.Args[0])
}
