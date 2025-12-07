package collector

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"kpi-collector/internal/config"
	"kpi-collector/internal/output"
	"kpi-collector/internal/prometheus"
)

const HUNDRED_MILLIS_DURATION_BUFFER = 100 * time.Millisecond

// calculateTotalSamples calculates how many samples will run for a given frequency and duration
func calculateTotalSamples(frequency int, duration time.Duration) int {
	// First sample runs immediately at t=0, then every frequency seconds
	// For duration D and frequency F: samples at 0, F, 2F, ... up to < D
	durationSecs := int(duration.Seconds())
	return (durationSecs / frequency) + 1
}

// Run executes the KPI collection loop
func Run(kpis config.KPIs, flags config.InputFlags) {
	durationTimer := time.NewTimer(flags.Duration + HUNDRED_MILLIS_DURATION_BUFFER)
	defer durationTimer.Stop()

	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt)

	output.PrintStartup(flags.Duration.String(), time.Now().Add(flags.Duration).Format(time.RFC3339))

	// Start all KPI goroutines grouped by frequency
	cancel, wg := startKPIGoroutines(kpis, flags)
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
}

// groupKPIsByFrequency groups KPIs by their effective sampling frequency
// This includes both default and custom frequency KPIs
func groupKPIsByFrequency(kpis config.KPIs, defaultFreq int) map[int]config.KPIs {
	kpisByFreq := make(map[int]config.KPIs)

	for _, kpi := range kpis.Queries {
		effectiveFreq := kpi.GetEffectiveFrequency(defaultFreq)

		group := kpisByFreq[effectiveFreq]
		group.Queries = append(group.Queries, kpi)
		kpisByFreq[effectiveFreq] = group
	}

	if len(kpisByFreq) > 0 {
		log.Printf("Grouped all KPIs into %d unique frequency groups", len(kpisByFreq))
		for freq, group := range kpisByFreq {
			log.Printf("  Frequency %ds: %d KPIs", freq, len(group.Queries))
		}
	}

	return kpisByFreq
}

// startKPIGoroutines starts one goroutine per unique frequency for all KPIs
func startKPIGoroutines(kpis config.KPIs, flags config.InputFlags) (context.CancelFunc, *sync.WaitGroup) {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	// Group ALL KPIs by their sampling frequency (including default frequency)
	kpisByFreq := groupKPIsByFrequency(kpis, flags.SamplingFreq)

	// Start one goroutine per unique frequency
	for freq, kpisForFreq := range kpisByFreq {
		wg.Add(1)
		go func(frequency int, kpiGroup config.KPIs) {
			defer wg.Done()
			runKPIGroupLoop(ctx, kpiGroup, frequency, flags)
		}(freq, kpisForFreq)
	}

	return cancel, &wg
}

func shutdown(cancel context.CancelFunc, wg *sync.WaitGroup) {
	cancel()
	wg.Wait()
}

// runKPIGroupLoop runs a group of KPIs that share the same sampling frequency
func runKPIGroupLoop(ctx context.Context, kpis config.KPIs, frequency int, flags config.InputFlags) {
	ticker := time.NewTicker(time.Duration(frequency) * time.Second)
	defer ticker.Stop()

	sampleCount := 0
	totalSamples := calculateTotalSamples(frequency, flags.Duration)
	log.Printf("Starting goroutine for %d KPIs with frequency %ds (total samples: %d)", len(kpis.Queries), frequency, totalSamples)

	// Run immediately on start
	sampleCount++
	runKPIs(kpis, flags, sampleCount, totalSamples, frequency)

	for {
		select {
		case <-ticker.C:
			sampleCount++
			runKPIs(kpis, flags, sampleCount, totalSamples, frequency)

		case <-ctx.Done():
			log.Printf("KPI group (frequency %ds) stopped after %d samples", frequency, sampleCount)
			return
		}
	}
}

// runKPIs executes a group of KPIs and logs the results
func runKPIs(kpis config.KPIs, flags config.InputFlags, sampleNumber int, totalSamples int, frequency int) {
	if len(kpis.Queries) == 0 {
		return
	}

	log.Printf("Running sample %d/%d for %d KPIs with frequency %ds", sampleNumber, totalSamples, len(kpis.Queries), frequency)

	if err := prometheus.RunQueries(kpis, flags, sampleNumber, totalSamples, frequency); err != nil {
		log.Printf("RunQueries failed for frequency %ds KPIs: %v", frequency, err)
	}
}
