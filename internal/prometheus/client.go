package prometheus

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"kpi-collector/internal/config"
	"kpi-collector/internal/database"
	"kpi-collector/internal/output"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

const (
	FIVE_SECONDS = 5 * time.Second
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
func RunQueries(kpisToRun config.KPIs, flags config.InputFlags, sampleNumber int, totalSamples int, frequency int) error {
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

	// Execute queries
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(len(kpisToRun.Queries))*FIVE_SECONDS)
	defer cancel()

	for _, query := range kpisToRun.Queries {
		queryInfo := output.QueryInfo{
			QueryID:      query.ID,
			PromQuery:    query.PromQuery,
			Frequency:    frequency,
			SampleNumber: sampleNumber,
			TotalSamples: totalSamples,
		}
		executeQuery(ctx, v1api, db, dbImpl, clusterID, queryInfo)
	}

	return nil
}

// executeQuery executes a single Prometheus query and handles the result
func executeQuery(ctx context.Context, v1api promv1.API, db *sql.DB, dbImpl database.Database, clusterID int64, info output.QueryInfo) {
	// Execute query using the Prometheus client library
	result, warnings, err := v1api.Query(ctx, info.PromQuery, time.Now())

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
}
