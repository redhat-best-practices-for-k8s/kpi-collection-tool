// Package collector orchestrates KPI metric collection from Prometheus/Thanos.
// It manages concurrent collection of KPIs grouped by sampling frequency,
// handles graceful shutdown, and coordinates with the prometheus package
// for query execution and storage.
package collector

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/database"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/output"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/prometheus"
)

// durationBuffer adds a small grace period to the duration timer so that when
// duration is an exact multiple of frequency, the final tick fires and its
// collection completes before the timer expires.
const durationBuffer = 100 * time.Millisecond

// RunOnce executes every KPI query exactly once and returns.
// It ignores frequency and duration settings entirely.
// Returns an error if any queries failed.
func RunOnce(kpis config.KPIs, flags config.InputFlags) error {
	db, dbImpl, err := database.InitDatabaseWithConfig(flags.DatabaseType, flags.PostgresURL)
	if err != nil {
		return fmt.Errorf("failed to init database: %v", err)
	}
	defer closeDB(db)

	fmt.Printf("\nKPI Collection Started - Single run mode\n")
	log.Printf("Single run: executing %d KPIs", len(kpis.Queries))

	err = prometheus.RunQueries(db, dbImpl, kpis, flags, 1, 1, 0)
	if err != nil {
		log.Printf("RunQueries failed in single-run mode: %v", err)
	}

	output.PrintShutdown("Single run completed")
	return err
}

// Run executes the KPI collection loop until duration expires or interrupted.
// Returns an error if any query failures occurred during collection.
func Run(kpis config.KPIs, flags config.InputFlags) error {
	db, dbImpl, err := database.InitDatabaseWithConfig(flags.DatabaseType, flags.PostgresURL)
	if err != nil {
		return fmt.Errorf("failed to init database: %v", err)
	}
	defer closeDB(db)

	var hadFailures atomic.Bool

	runOnceKPIs, repeatingKPIs := splitRunOnceQueries(kpis)

	// Execute run-once queries immediately before starting the loop
	if len(runOnceKPIs.Queries) > 0 {
		fmt.Printf("Executing %d run-once KPI(s) before starting collection loop\n", len(runOnceKPIs.Queries))
		log.Printf("Executing %d run-once KPIs", len(runOnceKPIs.Queries))

		if err := prometheus.RunQueries(db, dbImpl, runOnceKPIs, flags, 1, 1, 0); err != nil {
			log.Printf("RunQueries failed for run-once KPIs: %v", err)
			hadFailures.Store(true)
		}
	}

	if len(repeatingKPIs.Queries) == 0 {
		output.PrintShutdown("All queries are run-once, collection complete")
		if hadFailures.Load() {
			return fmt.Errorf("some queries failed during collection")
		}
		return nil
	}

	durationTimer := time.NewTimer(flags.Duration + durationBuffer)
	defer durationTimer.Stop()

	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt)

	output.PrintStartup(flags.Duration.String(), time.Now().Add(flags.Duration).Format(time.RFC3339))

	// Start repeating KPI goroutines grouped by frequency
	cancel, wg := startKPIGoroutines(db, dbImpl, repeatingKPIs, flags, &hadFailures)
	defer cancel()

	// Main goroutine only handles duration timer and interrupts
	var shutdownReason string
	select {
	case <-durationTimer.C:
		log.Printf("Duration timer expired")
		shutdownReason = "Duration completed"

	case <-interruptChan:
		log.Printf("Program interrupted")
		shutdownReason = "Interrupted by user"
	}

	// Wait for all goroutines to finish, then print shutdown message
	shutdown(cancel, wg)
	output.PrintShutdown(shutdownReason)

	if hadFailures.Load() {
		return fmt.Errorf("some queries failed during collection")
	}
	return nil
}

// splitRunOnceQueries separates KPIs into run-once and repeating groups
func splitRunOnceQueries(kpis config.KPIs) (runOnce config.KPIs, repeating config.KPIs) {
	for _, kpi := range kpis.Queries {
		if kpi.IsRunOnce() {
			runOnce.Queries = append(runOnce.Queries, kpi)
		} else {
			repeating.Queries = append(repeating.Queries, kpi)
		}
	}

	return runOnce, repeating
}

// groupKPIsByFrequency groups KPIs by their effective sampling frequency.
// This includes both default and custom frequency KPIs.
func groupKPIsByFrequency(kpis config.KPIs, defaultFreq time.Duration) map[time.Duration]config.KPIs {
	kpisByFreq := make(map[time.Duration]config.KPIs)

	for _, kpi := range kpis.Queries {
		effectiveFreq := kpi.GetEffectiveFrequency(defaultFreq)

		group := kpisByFreq[effectiveFreq]
		group.Queries = append(group.Queries, kpi)
		kpisByFreq[effectiveFreq] = group
	}

	if len(kpisByFreq) > 0 {
		log.Printf("Grouped all KPIs into %d unique frequency groups", len(kpisByFreq))
		for freq, group := range kpisByFreq {
			log.Printf("  Frequency %s: %d KPIs", freq, len(group.Queries))
		}
	}

	return kpisByFreq
}

// startKPIGoroutines starts one goroutine per unique frequency for all KPIs
func startKPIGoroutines(db *sql.DB, dbImpl database.Database, kpis config.KPIs,
	flags config.InputFlags, hadFailures *atomic.Bool) (context.CancelFunc, *sync.WaitGroup) {

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	// Group ALL KPIs by their sampling frequency (including default frequency)
	kpisByFreq := groupKPIsByFrequency(kpis, flags.SamplingFreq)

	// Start one goroutine per unique frequency
	for freq, kpisForFreq := range kpisByFreq {
		wg.Add(1)
		go func(frequency time.Duration, kpiGroup config.KPIs) {
			defer wg.Done()
			runKPIGroupLoop(ctx, db, dbImpl, kpiGroup, frequency, flags, hadFailures)
		}(freq, kpisForFreq)
	}

	return cancel, &wg
}

func shutdown(cancel context.CancelFunc, wg *sync.WaitGroup) {
	cancel()
	wg.Wait()
}

// runKPIGroupLoop runs a group of KPIs that share the same sampling frequency
func runKPIGroupLoop(ctx context.Context, db *sql.DB, dbImpl database.Database,
	kpis config.KPIs, frequency time.Duration, flags config.InputFlags, hadFailures *atomic.Bool) {

	ticker := time.NewTicker(frequency)
	defer ticker.Stop()

	sampleCount := 0
	totalSamples := calculateTotalSamples(frequency, flags.Duration)
	log.Printf("Starting goroutine for %d KPIs with frequency %s (total samples: %d)", len(kpis.Queries), frequency, totalSamples)
	// Run immediately on start
	sampleCount++
	runKPIs(db, dbImpl, kpis, flags, sampleCount, totalSamples, frequency, hadFailures)

	for {
		select {
		case <-ticker.C:
			sampleCount++
			runKPIs(db, dbImpl, kpis, flags, sampleCount, totalSamples, frequency, hadFailures)

		case <-ctx.Done():
			log.Printf("KPI group (frequency %s) stopped after %d samples", frequency, sampleCount)
			return
		}
	}
}

// runKPIs executes a group of KPIs and logs the results
func runKPIs(db *sql.DB, dbImpl database.Database, kpis config.KPIs, flags config.InputFlags,
	sampleNumber int, totalSamples int, frequency time.Duration, hadFailures *atomic.Bool) {

	if len(kpis.Queries) == 0 {
		return
	}

	log.Printf("Running sample %d/%d for %d KPIs with frequency %s", sampleNumber, totalSamples, len(kpis.Queries), frequency)

	if err := prometheus.RunQueries(db, dbImpl, kpis, flags, sampleNumber, totalSamples, frequency); err != nil {
		log.Printf("RunQueries failed for frequency %s KPIs: %v", frequency, err)
		hadFailures.Store(true)
	}
}

// calculateTotalSamples calculates how many samples will run for a given frequency and duration.
// First sample runs immediately at t=0, then every frequency seconds.
// For duration D and frequency F: samples at 0, F, 2F, ... up to < D
func calculateTotalSamples(frequency time.Duration, duration time.Duration) int {
	frequencySecs := int(frequency.Seconds())
	durationSecs := int(duration.Seconds())
	return (durationSecs / frequencySecs) + 1
}

func closeDB(db *sql.DB) {
	if err := db.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to close database: %v\n", err)
	}
}
