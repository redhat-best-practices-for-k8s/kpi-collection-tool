package output

import (
	"fmt"
	"sync"
	"time"
)

var printMutex sync.Mutex

// QueryInfo holds information about a query execution for printing
type QueryInfo struct {
	QueryID      string
	PromQuery    string
	Frequency    time.Duration
	SampleNumber int
	TotalSamples int
	QueryType    string
	Step         time.Duration
	Since        time.Time
	Until        time.Time
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

	if info.Frequency == 0 {
		fmt.Printf("[%s] Sample %d/%d (single run)\n", info.QueryID, info.SampleNumber, info.TotalSamples)
	} else {
		fmt.Printf("[%s] Sample %d/%d (freq: %s)\n", info.QueryID, info.SampleNumber, info.TotalSamples, info.Frequency)
	}

	fmt.Printf("  Query: %s\n", info.PromQuery)
	if info.QueryType == "range" {
		fmt.Printf("  Query Type: range (step: %s, since: %s, until: %s)\n",
			info.Step, info.Since.Format(time.RFC3339), info.Until.Format(time.RFC3339))
	} else {
		fmt.Printf("  Query Type: instant\n")
	}

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
}

// PrintShutdown prints collection shutdown info (thread-safe)
func PrintShutdown(reason string) {
	printMutex.Lock()
	defer printMutex.Unlock()

	fmt.Println()
	fmt.Printf("KPI Collection Stopped: %s\n", reason)
}
