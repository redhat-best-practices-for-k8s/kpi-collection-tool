package database

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/prometheus/common/model"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
)

// PostgresDB implements the Database interface for PostgreSQL
type PostgresDB struct {
	ConnectionURL string
}

// NewPostgresDB creates a new PostgreSQL database instance
func NewPostgresDB(connectionURL string) *PostgresDB {
	return &PostgresDB{ConnectionURL: connectionURL}
}

// InitDB initializes the PostgreSQL database and creates required tables
func (p *PostgresDB) InitDB() (*sql.DB, error) {
	db, err := sql.Open("postgres", p.ConnectionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %v", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %v", err)
	}

	schema := `
    CREATE TABLE IF NOT EXISTS clusters (
        id SERIAL PRIMARY KEY,
        cluster_name TEXT UNIQUE NOT NULL,
		cluster_type TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    
    CREATE TABLE IF NOT EXISTS query_results (
        id SERIAL PRIMARY KEY,
        kpi_id TEXT NOT NULL,
        metric_value DOUBLE PRECISION,
        timestamp_value DOUBLE PRECISION,
        cluster_id INTEGER NOT NULL REFERENCES clusters(id),
        execution_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        metric_labels JSONB
    );

    CREATE TABLE IF NOT EXISTS query_errors (
        id SERIAL PRIMARY KEY,
        kpi_id TEXT UNIQUE NOT NULL,
        errors INTEGER DEFAULT 0
    );

    -- Create indexes for better query performance
    CREATE INDEX IF NOT EXISTS idx_query_results_cluster_id ON query_results(cluster_id);
    CREATE INDEX IF NOT EXISTS idx_query_results_kpi_id ON query_results(kpi_id);
    CREATE INDEX IF NOT EXISTS idx_query_results_created_at ON query_results(created_at);
    CREATE INDEX IF NOT EXISTS idx_query_results_labels ON query_results USING GIN(metric_labels);

    -- Prevent duplicate data points from overlapping range query windows
    CREATE UNIQUE INDEX IF NOT EXISTS idx_query_results_dedup
    ON query_results(kpi_id, cluster_id, timestamp_value, metric_labels);
    `

	_, err = db.Exec(schema)
	return db, err
}

// GetOrCreateCluster gets existing cluster ID or creates a new cluster record
func (p *PostgresDB) GetOrCreateCluster(db *sql.DB, clusterName string, clusterType string) (int64, error) {
	var clusterID int64

	// Try to get existing cluster
	err := db.QueryRow("SELECT id FROM clusters WHERE cluster_name = $1", clusterName).Scan(&clusterID)
	if err == nil {
		if clusterType != "" {
			_, updateErr := db.Exec("UPDATE clusters SET cluster_type = $1 WHERE id = $2", clusterType, clusterID)
			if updateErr != nil {
				return clusterID, updateErr
			}
		}
		return clusterID, nil
	}

	// Insert new cluster and return ID
	err = db.QueryRow(
		"INSERT INTO clusters (cluster_name, cluster_type) VALUES ($1, $2) ON CONFLICT (cluster_name) DO UPDATE SET cluster_type = EXCLUDED.cluster_type RETURNING id",
		clusterName, clusterType,
	).Scan(&clusterID)

	return clusterID, err
}

// IncrementQueryError increments the error count for a given KPI ID
func (p *PostgresDB) IncrementQueryError(db *sql.DB, kpiID string) error {
	_, err := db.Exec(`
        INSERT INTO query_errors (kpi_id, errors) VALUES ($1, 1)
        ON CONFLICT(kpi_id) DO UPDATE SET errors = query_errors.errors + 1
    `, kpiID)
	return err
}

// GetQueryErrorCount returns the error count for a given KPI ID
func (p *PostgresDB) GetQueryErrorCount(db *sql.DB, kpiID string) (int, error) {
	var count int
	err := db.QueryRow("SELECT errors FROM query_errors WHERE kpi_id = $1", kpiID).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return count, err
}

// EnsureCategoryTable is a no-op stub for PostgreSQL. Category table support
// will be implemented in a future PR; all writes go to query_results for now.
// --- TODO ---
func (p *PostgresDB) EnsureCategoryTable(_ *sql.DB, _ string, _ string) (string, error) {
	return DefaultTableName, nil
}

// ValidateCategoryConsistency is a no-op stub for PostgreSQL until category
// table support is implemented.
// --- TODO ---
func (p *PostgresDB) ValidateCategoryConsistency(_ *sql.DB, _ []config.Query) error {
	return nil
}

// ListCategories is a no-op stub for PostgreSQL until category support is implemented.
// --- TODO ---
func (p *PostgresDB) ListCategories(_ *sql.DB) ([]CategoryInfo, error) {
	return nil, nil
}

// LookupCategoryForKPI is a no-op stub for PostgreSQL until category support is implemented.
// --- TODO ---
func (p *PostgresDB) LookupCategoryForKPI(_ *sql.DB, _ string) (string, string, error) {
	return "", "", nil
}

// DeleteByCategory is a no-op stub for PostgreSQL until category support is implemented.
// --- TODO ---
func (p *PostgresDB) DeleteByCategory(_ *sql.DB, _ string) (int64, error) {
	return 0, nil
}

// StoreQueryResults stores the results of a Prometheus query in the database.
// The category parameter is accepted for interface compatibility but ignored —
// all data is written to the default query_results table until PostgreSQL
// category support is implemented.
func (p *PostgresDB) StoreQueryResults(db *sql.DB, clusterID int64, queryID string, _ string, result model.Value) error {
	switch values := result.(type) {
	case model.Vector:
		return p.storeVectorResults(db, clusterID, queryID, values)
	case model.Matrix:
		return p.storeMatrixResults(db, clusterID, queryID, values)
	default:
		return fmt.Errorf("unsupported Prometheus result type for KPI '%s': %T", queryID, result)
	}
}

func (p *PostgresDB) storeVectorResults(db *sql.DB, clusterID int64, queryID string, vector model.Vector) error {
	for _, sample := range vector {
		metric := sample.Metric
		value := float64(sample.Value)
		timestamp := float64(sample.Timestamp) / 1000

		labelsJSON, err := json.Marshal(metric)
		if err != nil {
			return err
		}

		_, err = db.Exec(`
            INSERT INTO query_results 
            (kpi_id, metric_value, timestamp_value, cluster_id, metric_labels)
            VALUES ($1, $2, $3, $4, $5)
            ON CONFLICT (kpi_id, cluster_id, timestamp_value, metric_labels) DO NOTHING`,
			queryID, value, timestamp, clusterID, string(labelsJSON),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *PostgresDB) storeMatrixResults(db *sql.DB, clusterID int64, queryID string, matrix model.Matrix) error {
	for _, stream := range matrix {
		metric := stream.Metric
		labelsJSON, err := json.Marshal(metric)
		if err != nil {
			return err
		}

		for _, samplePair := range stream.Values {
			value := float64(samplePair.Value)
			timestamp := float64(samplePair.Timestamp) / 1000

			_, execErr := db.Exec(`
                INSERT INTO query_results 
                (kpi_id, metric_value, timestamp_value, cluster_id, metric_labels)
                VALUES ($1, $2, $3, $4, $5)
                ON CONFLICT (kpi_id, cluster_id, timestamp_value, metric_labels) DO NOTHING`,
				queryID, value, timestamp, clusterID, string(labelsJSON),
			)
			if execErr != nil {
				return execErr
			}
		}
	}

	return nil
}
