package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/prometheus/common/model"
	_ "modernc.org/sqlite"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/database/schema"
)

const (
	// DefaultOutputDir is the default artifacts directory name, relative to CWD
	DefaultOutputDir = "kpi-collector-artifacts"
	// DefaultDBFileName is the SQLite database file name
	DefaultDBFileName = "kpi_metrics.db"
)

// OutputDir is the resolved artifacts directory. It defaults to DefaultOutputDir
// and can be overridden via the --artifacts-dir flag.
var OutputDir = DefaultOutputDir

type SQLiteDB struct{}

// NewSQLiteDB creates a new SQLite database instance
func NewSQLiteDB() *SQLiteDB {
	return &SQLiteDB{}
}

// InitDB initializes the SQLite database and creates required tables.
// The database is stored in <OutputDir>/kpi_metrics.db.
func (sqlite_db *SQLiteDB) InitDB() (*sql.DB, error) {
	dbPath := filepath.Join(OutputDir, DefaultDBFileName)

	if err := os.MkdirAll(OutputDir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if _, err = db.Exec(schema.SQLitePragmas); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("setting SQLite pragmas: %w", err)
	}

	if _, err = db.Exec(schema.SQLiteSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("creating SQLite schema: %w", err)
	}

	return db, nil
}

// getOrCreateCluster gets existing cluster ID or creates a new cluster record
func (sqlite_db *SQLiteDB) GetOrCreateCluster(db *sql.DB, clusterName string, clusterType string) (int64, error) {
	var clusterID int64
	err := db.QueryRow(schema.SQLiteSelectClusterByName, clusterName).Scan(&clusterID)
	if err == nil {
		if clusterType != "" {
			_, updateErr := db.Exec(schema.SQLiteUpdateClusterType, clusterType, clusterID)
			if updateErr != nil {
				return clusterID, updateErr
			}
		}
		return clusterID, nil
	}

	result, err := db.Exec(schema.SQLiteInsertCluster, clusterName, clusterType)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// increments the error count for a given KPI ID in the query_errors table.
func (sqlite_db *SQLiteDB) IncrementQueryError(db *sql.DB, kpiID string) error {
	_, err := db.Exec(schema.SQLiteUpsertQueryError, kpiID)
	return err
}

// returns the error count for a given KPI ID.
func (sqlite_db *SQLiteDB) GetQueryErrorCount(db *sql.DB, kpiID string) (int, error) {
	var count int
	err := db.QueryRow(schema.SQLiteSelectErrorCount, kpiID).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return count, err
}

// StoreQueryResults stores the results of a Prometheus query in the database
func (sqlite_db *SQLiteDB) StoreQueryResults(db *sql.DB, clusterID int64, queryID string, result model.Value) error {
	switch values := result.(type) {
	case model.Vector:
		return sqlite_db.storeVectorResults(db, clusterID, queryID, values)
	case model.Matrix:
		return sqlite_db.storeMatrixResults(db, clusterID, queryID, values)
	default:
		return fmt.Errorf("unsupported Prometheus result type for KPI '%s': %T", queryID, result)
	}
}

func (sqlite_db *SQLiteDB) storeVectorResults(db *sql.DB, clusterID int64, queryID string, vector model.Vector) error {
	transaction, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	preparedStatement, err := transaction.Prepare(schema.SQLiteInsertResult)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer func() { _ = preparedStatement.Close() }()

	for _, sample := range vector {
		labelsJSON, err := json.Marshal(sample.Metric)
		if err != nil {
			return err
		}
		if _, err = preparedStatement.Exec(
			queryID,
			float64(sample.Value),
			float64(sample.Timestamp)/1000,
			clusterID,
			string(labelsJSON),
		); err != nil {
			return err
		}
	}

	return transaction.Commit()
}

func (sqlite_db *SQLiteDB) storeMatrixResults(db *sql.DB, clusterID int64, queryID string, matrix model.Matrix) error {
	transaction, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	preparedStatement, err := transaction.Prepare(schema.SQLiteInsertResult)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer func() { _ = preparedStatement.Close() }()

	for _, stream := range matrix {
		labelsJSON, err := json.Marshal(stream.Metric)
		if err != nil {
			return err
		}

		for _, samplePair := range stream.Values {
			if _, err = preparedStatement.Exec(
				queryID,
				float64(samplePair.Value),
				float64(samplePair.Timestamp)/1000,
				clusterID,
				string(labelsJSON),
			); err != nil {
				return err
			}
		}
	}

	return transaction.Commit()
}
