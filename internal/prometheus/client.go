// Package prometheus provides client functionality for querying Prometheus/Thanos
// metrics endpoints. It handles HTTP client configuration with bearer token
// authentication, TLS settings, and executes PromQL queries with results
// stored via the database package.
package prometheus

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/database"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/output"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

const (
	// queryTimeoutPerKPI is the maximum time allowed for each individual KPI query.
	// The total context timeout is calculated as: numberOfQueries * queryTimeoutPerKPI
	queryTimeoutPerKPI = 5 * time.Second
)

// setupPromClient creates and configures a Prometheus API client
func setupPromClient(thanosURL, bearerToken string, insecureTLS bool) (promv1.API, error) {
	client, err := api.NewClient(api.Config{
		Address: "https://" + thanosURL,
		RoundTripper: &tokenRoundTripper{
			Token: bearerToken,
			RT: &http.Transport{
				// NOTE: InsecureSkipVerify is set to true for development purposes only.
				// In production environments, this should be false and proper certificate
				// validation should be implemented.
				TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureTLS},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus client: %v", err)
	}

	return promv1.NewAPI(client), nil
}

// RunQueries executes all Prometheus queries and stores results in database
func RunQueries(kpisToRun config.KPIs, flags config.InputFlags, sampleNumber int, totalSamples int, frequency time.Duration) error {
	// Initialize Database based on configuration
	db, dbImpl, err := database.InitDatabaseWithConfig(flags.DatabaseType, flags.PostgresURL)
	if err != nil {
		return fmt.Errorf("failed to init database: %v", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close database: %v\n", closeErr)
		}
	}()

	// Get or create cluster in DB
	clusterID, err := dbImpl.GetOrCreateCluster(db, flags.ClusterName, flags.ClusterType)
	if err != nil {
		return fmt.Errorf("failed to get cluster ID: %v", err)
	}

	// Create Prometheus client
	v1api, err := setupPromClient(flags.ThanosURL, flags.BearerToken, flags.InsecureTLS)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	// Execute queries with a timeout proportional to the number of queries
	totalTimeout := time.Duration(len(kpisToRun.Queries)) * queryTimeoutPerKPI
	ctx, cancel := context.WithTimeout(context.Background(), totalTimeout)
	defer cancel()

	for _, query := range kpisToRun.Queries {
		queryInfo := output.QueryInfo{
			QueryID:      query.ID,
			PromQuery:    query.PromQuery,
			Frequency:    frequency,
			SampleNumber: sampleNumber,
			TotalSamples: totalSamples,
			QueryType:    query.GetEffectiveQueryType(),
		}
		if query.Range != nil {
			now := time.Now()
			if query.Range.Step != nil {
				queryInfo.Step = query.Range.Step.Duration
			}
			if query.Range.Since != nil {
				queryInfo.Since = query.Range.Since.Resolve(now)
			}
			if query.Range.Until != nil {
				queryInfo.Until = query.Range.Until.Resolve(now)
			} else {
				queryInfo.Until = now
			}
		}
		executeQuery(ctx, v1api, db, dbImpl, clusterID, queryInfo)
	}

	return nil
}

// executeQuery executes a single Prometheus query and handles the result
func executeQuery(ctx context.Context, v1api promv1.API, db *sql.DB, dbImpl database.Database, clusterID int64, info output.QueryInfo) {
	now := time.Now()

	var (
		result   model.Value
		warnings promv1.Warnings
		err      error
	)

	// Execute query using the Prometheus client library
	if info.QueryType == "range" {
		queryRange := promv1.Range{
			Start: info.Since,
			End:   info.Until,
			Step:  info.Step,
		}
		result, warnings, err = v1api.QueryRange(ctx, info.PromQuery, queryRange)
	} else {
		result, warnings, err = v1api.Query(ctx, info.PromQuery, now)
	}

	queryResult := output.QueryResult{
		Warnings: warnings,
	}

	if err != nil {
		queryResult.Success = false
		queryResult.Error = err
		output.PrintQueryResult(info, queryResult)
		if storeErr := dbImpl.IncrementQueryError(db, info.QueryID); storeErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to increment error count: %v\n", storeErr)
		}
		return
	}

	// Filter out NaN/Inf values
	var nanCount int
	result, nanCount = filterNaNValues(result)

	// Check if anything remains to store
	if isEmptyResult(result) {
		queryResult.Success = true
		output.PrintQueryResult(info, queryResult)
		if nanCount > 0 {
			fmt.Printf("  Warning: all %d sample(s) were NaN — nothing stored\n", nanCount)
			log.Printf("[%s] All %d sample(s) were NaN, nothing stored: %s", info.QueryID, nanCount, info.PromQuery)
		} else {
			fmt.Printf("  Warning: query returned no data (metric may not exist on this cluster)\n")
			log.Printf("[%s] Query returned no data: %s", info.QueryID, info.PromQuery)
		}
		return
	}

	// Store results
	err = dbImpl.StoreQueryResults(db, clusterID, info.QueryID, result)
	if err != nil {
		queryResult.Success = false
		queryResult.Error = fmt.Errorf("failed to store: %v", err)
		output.PrintQueryResult(info, queryResult)
		return
	}

	queryResult.Success = true
	output.PrintQueryResult(info, queryResult)
	if nanCount > 0 {
		fmt.Printf("  Note: skipped %d NaN value(s)\n", nanCount)
		log.Printf("[%s] Skipped %d NaN/Inf value(s): %s", info.QueryID, nanCount, info.PromQuery)
	}
}

// isEmptyResult checks whether a Prometheus query returned no data points.
func isEmptyResult(result model.Value) bool {
	switch v := result.(type) {
	case model.Vector:
		return len(v) == 0
	case model.Matrix:
		return len(v) == 0
	default:
		return false
	}
}

// filterNaNValues removes NaN and Inf samples from a Prometheus result,
// returning the cleaned result and the number of samples removed.
func filterNaNValues(result model.Value) (model.Value, int) {
	switch v := result.(type) {
	case model.Vector:
		filtered := make(model.Vector, 0, len(v))
		for _, sample := range v {
			if math.IsNaN(float64(sample.Value)) || math.IsInf(float64(sample.Value), 0) {
				continue
			}
			filtered = append(filtered, sample)
		}
		return filtered, len(v) - len(filtered)

	case model.Matrix:
		skipped := 0
		filtered := make(model.Matrix, 0, len(v))
		for _, stream := range v {
			cleanPairs := make([]model.SamplePair, 0, len(stream.Values))
			for _, pair := range stream.Values {
				if math.IsNaN(float64(pair.Value)) || math.IsInf(float64(pair.Value), 0) {
					skipped++
					continue
				}
				cleanPairs = append(cleanPairs, pair)
			}
			if len(cleanPairs) > 0 {
				filtered = append(filtered, &model.SampleStream{
					Metric: stream.Metric,
					Values: cleanPairs,
				})
			}
		}
		return filtered, skipped

	default:
		return result, 0
	}
}
