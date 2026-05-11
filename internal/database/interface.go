// Package database provides a unified interface for storing and retrieving
// KPI metrics. It supports multiple backends including SQLite for local
// storage and PostgreSQL for production deployments, with consistent
// behavior across implementations.
package database

import (
	"database/sql"

	"github.com/prometheus/common/model"
)

// Database defines the interface that all database implementations must satisfy
type Database interface {
	// InitDB initializes the database and creates required tables
	InitDB() (*sql.DB, error)

	// GetOrCreateCluster gets existing cluster ID or creates a new cluster record
	GetOrCreateCluster(db *sql.DB, clusterName string, clusterType string) (int64, error)

	// IncrementQueryError increments the error count for a given KPI ID
	IncrementQueryError(db *sql.DB, kpiID string) error

	// GetQueryErrorCount returns the error count for a given KPI ID
	GetQueryErrorCount(db *sql.DB, kpiID string) (int, error)

	// StoreQueryResults stores the results of a Prometheus query in the database.
	// When category is non-empty, data is written to a category-specific table
	// (e.g. kpi_cpu); otherwise it goes to the default query_results table.
	// Supports model.Vector (from instant queries) and model.Matrix (from range queries).
	StoreQueryResults(db *sql.DB, clusterID int64, queryID string, category string, result model.Value) error

	// EnsureCategoryTable creates the per-category table and dedup index if they
	// don't already exist, and registers the KPI→category mapping in kpi_registry.
	EnsureCategoryTable(db *sql.DB, category string, kpiID string) (string, error)
}
