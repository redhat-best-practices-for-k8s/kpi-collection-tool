package output

import (
	"fmt"
	"sync"
)

var printMutex sync.Mutex

// QueryInfo holds information about a query execution for printing
type QueryInfo struct {
	QueryID      string
	PromQuery    string
	Frequency    int
	SampleNumber int
	TotalSamples int
}

// QueryResult holds the result of a query execution
type QueryResult struct {
	Success  bool
	Error    error
	Warnings []string
}

// PrintQueryResult prints the complete query execution result atomically (thread-safe)
func PrintQueryResult(info QueryInfo, result QueryResult) {
	printMutex.Lock()
	defer printMutex.Unlock()

	fmt.Println()
	fmt.Printf("[%s] Sample %d/%d (freq: %ds)\n", info.QueryID, info.SampleNumber, info.TotalSamples, info.Frequency)
	fmt.Printf("  Query: %s\n", info.PromQuery)

	if len(result.Warnings) > 0 {
		fmt.Printf("  Warnings: %v\n", result.Warnings)
	}

	if result.Success {
		fmt.Printf("  Status: OK - stored in database\n")
	} else {
		fmt.Printf("  Status: FAILED - %v\n", result.Error)
	}
}

// PrintStartup prints collection startup info (thread-safe)
func PrintStartup(duration string, deadline string) {
	printMutex.Lock()
	defer printMutex.Unlock()

	fmt.Println()
	fmt.Printf("KPI Collection Started - Duration: %s (until %s)\n", duration, deadline)
	fmt.Println()
}

// PrintShutdown prints collection shutdown info (thread-safe)
func PrintShutdown(reason string) {
	printMutex.Lock()
	defer printMutex.Unlock()

	fmt.Println()
	fmt.Printf("KPI Collection Stopped: %s\n", reason)
}
