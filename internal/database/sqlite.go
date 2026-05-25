package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/prometheus/common/model"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
	_ "modernc.org/sqlite"
)

const (
	// DefaultOutputDir is the default artifacts directory name, relative to CWD
	DefaultOutputDir = "kpi-collector-artifacts"
	// DefaultDBFileName is the SQLite database file name
	DefaultDBFileName = "kpi_metrics.db"
	// DefaultTableName is the legacy table used for uncategorized KPIs
	DefaultTableName = "query_results"
)

// OutputDir is the resolved artifacts directory. It defaults to DefaultOutputDir
// and can be overridden via the --artifacts-dir flag.
var OutputDir = DefaultOutputDir

type SQLiteDB struct {
	// knownTables caches which category tables have been created during this
	// process lifetime. It resets on every invocation (including --once), which
	// is fine because EnsureCategoryTable uses CREATE TABLE IF NOT EXISTS as the
	// fallback — the cache only avoids redundant DDL within a long-running run.
	knownTables sync.Map
}

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

	schema := `
    CREATE TABLE IF NOT EXISTS clusters (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        cluster_name TEXT UNIQUE NOT NULL,
		cluster_type TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    
    
    CREATE TABLE IF NOT EXISTS query_results (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        kpi_id TEXT NOT NULL,
        metric_value REAL,
        timestamp_value REAL,
		cluster_id INTEGER NOT NULL REFERENCES clusters(id),
		execution_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        metric_labels TEXT  -- JSON string of all labels
    );

	CREATE TABLE IF NOT EXISTS query_errors (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		kpi_id TEXT UNIQUE NOT NULL,
		errors INTEGER DEFAULT 0
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_query_results_dedup
	ON query_results(kpi_id, cluster_id, timestamp_value, metric_labels);

	CREATE TABLE IF NOT EXISTS kpi_registry (
		kpi_id TEXT PRIMARY KEY,
		category TEXT NOT NULL,
		table_name TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)
    `

	_, err = db.Exec(schema)
	return db, err
}

// getOrCreateCluster gets existing cluster ID or creates a new cluster record
func (sqlite_db *SQLiteDB) GetOrCreateCluster(db *sql.DB, clusterName string, clusterType string) (int64, error) {
	var clusterID int64
	err := db.QueryRow("SELECT id FROM clusters WHERE cluster_name = ?", clusterName).Scan(&clusterID)
	if err == nil {
		if clusterType != "" {
			_, updateErr := db.Exec("UPDATE clusters SET cluster_type = ? WHERE id = ?", clusterType, clusterID)
			if updateErr != nil {
				return clusterID, updateErr
			}
		}
		return clusterID, nil
	}

	result, err := db.Exec("INSERT INTO clusters (cluster_name, cluster_type) VALUES (?, ?)", clusterName, clusterType)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// increments the error count for a given KPI ID in the query_errors table.
func (sqlite_db *SQLiteDB) IncrementQueryError(db *sql.DB, kpiID string) error {
	_, err := db.Exec(`
        INSERT INTO query_errors (kpi_id, errors) VALUES (?, 1)
        ON CONFLICT(kpi_id) DO UPDATE SET errors = errors + 1
    `, kpiID)
	return err
}

// returns the error count for a given KPI ID.
func (sqlite_db *SQLiteDB) GetQueryErrorCount(db *sql.DB, kpiID string) (int, error) {
	var count int
	err := db.QueryRow("SELECT errors FROM query_errors WHERE kpi_id = ?", kpiID).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return count, err
}

// StoreQueryResults stores the results of a Prometheus query in the database.
// When category is non-empty, writes go to the per-category table (kpi_<category>).
func (sqlite_db *SQLiteDB) StoreQueryResults(db *sql.DB, clusterID int64, queryID string, category string, result model.Value) error {
	tableName := DefaultTableName
	if category != "" {
		name, err := sqlite_db.EnsureCategoryTable(db, category, queryID)
		if err != nil {
			return fmt.Errorf("ensure category table for '%s': %w", category, err)
		}
		tableName = name
	}

	switch values := result.(type) {
	case model.Vector:
		return sqlite_db.storeVectorResults(db, clusterID, queryID, tableName, values)
	case model.Matrix:
		return sqlite_db.storeMatrixResults(db, clusterID, queryID, tableName, values)
	default:
		return fmt.Errorf("unsupported Prometheus result type for KPI '%s': %T", queryID, result)
	}
}

// CategoryTableName returns the physical table name for a given sanitised category.
func CategoryTableName(category string) string {
	return "kpi_" + category
}

// EnsureCategoryTable lazily creates the per-category table and registers the
// KPI→category mapping. The DDL is idempotent and only executed once per
// process lifetime thanks to the in-memory knownTables cache.
func (sqlite_db *SQLiteDB) EnsureCategoryTable(db *sql.DB, category string, kpiID string) (string, error) {
	tableName := CategoryTableName(category)

	if _, ok := sqlite_db.knownTables.Load(tableName); !ok {
		ddl := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				kpi_id TEXT NOT NULL,
				metric_value REAL,
				timestamp_value REAL,
				cluster_id INTEGER NOT NULL REFERENCES clusters(id),
				execution_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				metric_labels TEXT
			);
			CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_dedup
			ON %s(kpi_id, cluster_id, timestamp_value, metric_labels)`,
			tableName, tableName, tableName)

		if _, err := db.Exec(ddl); err != nil {
			return "", fmt.Errorf("create table %s: %w", tableName, err)
		}

		sqlite_db.knownTables.Store(tableName, true)
	}

	_, err := db.Exec(`
		INSERT INTO kpi_registry (kpi_id, category, table_name) VALUES (?, ?, ?)
		ON CONFLICT(kpi_id) DO NOTHING`,
		kpiID, category, tableName)
	if err != nil {
		return "", fmt.Errorf("register kpi '%s' in kpi_registry: %w", kpiID, err)
	}

	return tableName, nil
}

// ValidateCategoryConsistency loads the existing kpi_registry and returns an
// error if any incoming KPI has a different category than what was previously
// recorded. This prevents silent data orphaning when a user changes categories
// between runs.
func (sqlite_db *SQLiteDB) ValidateCategoryConsistency(db *sql.DB, kpis []config.Query) error {
	rows, err := db.Query("SELECT kpi_id, category FROM kpi_registry")
	if err != nil {
		return fmt.Errorf("query kpi_registry: %w", err)
	}
	defer func() { _ = rows.Close() }()

	registered := make(map[string]string)
	for rows.Next() {
		var kpiID, category string
		if err := rows.Scan(&kpiID, &category); err != nil {
			return fmt.Errorf("scan kpi_registry row: %w", err)
		}
		registered[kpiID] = category
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate kpi_registry: %w", err)
	}

	for i := range kpis {
		prev, exists := registered[kpis[i].ID]
		if !exists {
			continue
		}
		if prev != kpis[i].Category {
			return fmt.Errorf(
				"KPI '%s' category changed from %q to %q — "+
					"use a different --artifacts-dir or delete the existing database",
				kpis[i].ID, prev, kpis[i].Category)
		}
	}

	return nil
}

// ListCategories returns all distinct categories registered in kpi_registry.
func (sqlite_db *SQLiteDB) ListCategories(db *sql.DB) ([]CategoryInfo, error) {
	rows, err := db.Query("SELECT DISTINCT category, table_name FROM kpi_registry ORDER BY category")
	if err != nil {
		return nil, fmt.Errorf("query kpi_registry: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var categories []CategoryInfo
	for rows.Next() {
		var ci CategoryInfo
		if err := rows.Scan(&ci.Category, &ci.TableName); err != nil {
			return nil, fmt.Errorf("scan kpi_registry row: %w", err)
		}
		categories = append(categories, ci)
	}

	return categories, rows.Err()
}

// LookupCategoryForKPI returns the category and table name for a KPI ID.
// Returns empty strings when the KPI has no registry entry (uncategorized).
func (sqlite_db *SQLiteDB) LookupCategoryForKPI(db *sql.DB, kpiID string) (string, string, error) {
	var category, tableName string
	err := db.QueryRow("SELECT category, table_name FROM kpi_registry WHERE kpi_id = ?", kpiID).
		Scan(&category, &tableName)

	if err == sql.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("lookup kpi_registry for '%s': %w", kpiID, err)
	}

	return category, tableName, nil
}

// DeleteByCategory removes all rows from the given category table and cleans
// up the corresponding kpi_registry entries. Returns the number of metric rows deleted.
func (sqlite_db *SQLiteDB) DeleteByCategory(db *sql.DB, category string) (int64, error) {
	tableName := CategoryTableName(category)

	result, err := db.Exec(fmt.Sprintf("DELETE FROM %s", tableName))
	if err != nil {
		return 0, fmt.Errorf("delete from %s: %w", tableName, err)
	}
	deleted, _ := result.RowsAffected()

	_, err = db.Exec("DELETE FROM kpi_registry WHERE category = ?", category)
	if err != nil {
		return deleted, fmt.Errorf("clean kpi_registry for category '%s': %w", category, err)
	}

	return deleted, nil
}

func (sqlite_db *SQLiteDB) storeVectorResults(db *sql.DB, clusterID int64, queryID string, table string, vector model.Vector) error {
	for _, sample := range vector {
		metric := sample.Metric
		value := float64(sample.Value)
		timestamp := float64(sample.Timestamp) / 1000

		labelsJSON, err := json.Marshal(metric)
		if err != nil {
			return err
		}

		_, err = db.Exec(fmt.Sprintf(`
            INSERT INTO %s
            (kpi_id, metric_value, timestamp_value, cluster_id, metric_labels)
            VALUES (?, ?, ?, ?, ?)
            ON CONFLICT(kpi_id, cluster_id, timestamp_value, metric_labels) DO NOTHING`, table),
			queryID, value, timestamp, clusterID, string(labelsJSON),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (sqlite_db *SQLiteDB) storeMatrixResults(db *sql.DB, clusterID int64, queryID string, table string, matrix model.Matrix) error {
	for _, stream := range matrix {
		metric := stream.Metric
		labelsJSON, err := json.Marshal(metric)
		if err != nil {
			return err
		}

		for _, samplePair := range stream.Values {
			value := float64(samplePair.Value)
			timestamp := float64(samplePair.Timestamp) / 1000

			_, execErr := db.Exec(fmt.Sprintf(`
                INSERT INTO %s
                (kpi_id, metric_value, timestamp_value, cluster_id, metric_labels)
                VALUES (?, ?, ?, ?, ?)
                ON CONFLICT(kpi_id, cluster_id, timestamp_value, metric_labels) DO NOTHING`, table),
				queryID, value, timestamp, clusterID, string(labelsJSON),
			)
			if execErr != nil {
				return execErr
			}
		}
	}

	return nil
}
