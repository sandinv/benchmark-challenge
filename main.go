package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"benchmark/internal/database"
)

type Config struct {
	DatabaseConn string
	Workers      int
	InputFile    string
}

func main() {

	config := parseFlags()

	parseConnectionString(config)

	reader, closeFun, err := parseInputFile(config.InputFile)
	if err != nil {
		panic(fmt.Sprintf("couldn't read input file: %s", err))
	}
	defer closeFun()

	db, err := database.Connect(config.DatabaseConn)
	if err != nil {
		panic(fmt.Sprintf("can't establish a connection with the database %s", err))
	}

	db.ConfigurePool(config.Workers)

	_ = reader
	_ = db

}

func parseFlags() Config {

	config := Config{}

	flag.IntVar(&config.Workers, "workers", 1, "number of concurrent workers")
	flag.StringVar(&config.InputFile, "inputFile", "", "CSV file path ( if not provided, reads from stdin")

	flag.Parse()

	return config
}

func parseConnectionString(config Config) {

	// Get connection string from environment variable
	config.DatabaseConn = os.Getenv("DATABASE_URL")

	if config.DatabaseConn == "" {
		// Provide a default for local development
		config.DatabaseConn = "postgresql://postgres:password@localhost:5432/homework?sslmode=disable"
		fmt.Fprintf(os.Stderr, "Warning: DATABASE_URL not set, using default: %s\n", config.DatabaseConn)
	}

}

func parseInputFile(filepath string) (io.Reader, func(), error) {

	if filepath != "" {
		inputF, err := os.Open(filepath)
		if err != nil {
			return nil, nil, err
		}

		return inputF, func() { inputF.Close() }, nil
	}

	return os.Stdin, func() {}, nil

}
