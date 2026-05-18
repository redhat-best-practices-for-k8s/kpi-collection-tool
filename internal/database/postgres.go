package database

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/prometheus/common/model"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/database/schema"
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

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %v", err)
	}

	if _, err = db.Exec(schema.PostgresSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("creating PostgreSQL schema: %w", err)
	}

	return db, nil
}

// GetOrCreateCluster gets existing cluster ID or creates a new cluster record
func (p *PostgresDB) GetOrCreateCluster(db *sql.DB, clusterName string, clusterType string) (int64, error) {
	var clusterID int64

	err := db.QueryRow(schema.PostgresSelectClusterByName, clusterName).Scan(&clusterID)
	if err == nil {
		if clusterType != "" {
			_, updateErr := db.Exec(schema.PostgresUpdateClusterType, clusterType, clusterID)
			if updateErr != nil {
				return clusterID, updateErr
			}
		}
		return clusterID, nil
	}

	err = db.QueryRow(schema.PostgresUpsertCluster, clusterName, clusterType).Scan(&clusterID)
	return clusterID, err
}

// IncrementQueryError increments the error count for a given KPI ID
func (p *PostgresDB) IncrementQueryError(db *sql.DB, kpiID string) error {
	_, err := db.Exec(schema.PostgresUpsertQueryError, kpiID)
	return err
}

// GetQueryErrorCount returns the error count for a given KPI ID
func (p *PostgresDB) GetQueryErrorCount(db *sql.DB, kpiID string) (int, error) {
	var count int
	err := db.QueryRow(schema.PostgresSelectErrorCount, kpiID).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return count, err
}

// StoreQueryResults stores the results of a Prometheus query in the database
func (p *PostgresDB) StoreQueryResults(db *sql.DB, clusterID int64, queryID string, result model.Value) error {
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
		labelsJSON, err := json.Marshal(sample.Metric)
		if err != nil {
			return err
		}

		_, err = db.Exec(schema.PostgresInsertResult,
			queryID, float64(sample.Value), float64(sample.Timestamp)/1000, clusterID, string(labelsJSON),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *PostgresDB) storeMatrixResults(db *sql.DB, clusterID int64, queryID string, matrix model.Matrix) error {
	for _, stream := range matrix {
		labelsJSON, err := json.Marshal(stream.Metric)
		if err != nil {
			return err
		}

		for _, samplePair := range stream.Values {
			_, execErr := db.Exec(schema.PostgresInsertResult,
				queryID, float64(samplePair.Value), float64(samplePair.Timestamp)/1000, clusterID, string(labelsJSON),
			)
			if execErr != nil {
				return execErr
			}
		}
	}

	return nil
}
