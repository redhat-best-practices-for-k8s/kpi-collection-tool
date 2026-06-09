package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/prometheus/common/model"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
	_ "modernc.org/sqlite"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/database/schema"
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
	knownTables sync.Map
}

// NewSQLiteDB creates a new SQLite database instance
func NewSQLiteDB() *SQLiteDB {
	return &SQLiteDB{}
}

// InitDB initializes the SQLite database and creates required tables.
// The database is stored in <OutputDir>/kpi_metrics.db.
func (s *SQLiteDB) InitDB() (*sql.DB, error) {
	dbPath := filepath.Join(OutputDir, DefaultDBFileName)

	if err := os.MkdirAll(OutputDir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// SQLite only supports one writer at a time. A single connection ensures
	// all pragmas (busy_timeout, WAL) stay in effect and prevents SQLITE_BUSY
	// when goroutines share this pool.
	db.SetMaxOpenConns(1)

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

// GetOrCreateCluster gets existing cluster ID or creates a new cluster record.
func (s *SQLiteDB) GetOrCreateCluster(db *sql.DB, clusterName string, clusterType string) (int64, error) {
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
		// Another goroutine may have inserted the same cluster concurrently.
		// Re-query instead of failing.
		retryErr := db.QueryRow(schema.SQLiteSelectClusterByName, clusterName).Scan(&clusterID)
		if retryErr != nil {
			return 0, fmt.Errorf("insert failed (%w) and retry select failed (%w)", err, retryErr)
		}
		return clusterID, nil
	}
	return result.LastInsertId()
}

// EnsureCategoryTable lazily creates the per-category table and registers the
// KPI->category mapping. The DDL is idempotent and only executed once per
// process lifetime thanks to the in-memory knownTables cache.
func (s *SQLiteDB) EnsureCategoryTable(db *sql.DB, category string, kpiID string) (string, error) {
	tableName := CategoryTableName(category)

	if _, ok := s.knownTables.Load(tableName); !ok {
		ddl := fmt.Sprintf(schema.SQLiteCategoryTableFmt, tableName, tableName, tableName)

		if _, err := db.Exec(ddl); err != nil {
			return "", fmt.Errorf("create table %s: %w", tableName, err)
		}

		s.knownTables.Store(tableName, true)
	}

	_, err := db.Exec(schema.SQLiteUpsertRegistry, kpiID, category, tableName)
	if err != nil {
		return "", fmt.Errorf("register kpi '%s' in kpi_registry: %w", kpiID, err)
	}

	return tableName, nil
}

// --- Thin wrappers delegating to shared free functions ---

func (s *SQLiteDB) IncrementQueryError(db *sql.DB, kpiID string) error {
	return incrementQueryError(db, kpiID, schema.SQLiteUpsertQueryError)
}

func (s *SQLiteDB) GetQueryErrorCount(db *sql.DB, kpiID string) (int, error) {
	return getQueryErrorCount(db, kpiID, schema.SQLiteSelectErrorCount)
}

func (s *SQLiteDB) StoreQueryResults(db *sql.DB, clusterID int64, queryID, category string, result model.Value) error {
	return storeQueryResults(db, clusterID, queryID, category, result, s.EnsureCategoryTable, schema.SQLiteInsertResultFmt)
}

func (s *SQLiteDB) ValidateCategoryConsistency(db *sql.DB, kpis []config.Query) error {
	return validateCategoryConsistency(db, kpis, "use a different --artifacts-dir or delete the existing database")
}

func (s *SQLiteDB) ListCategories(db *sql.DB) ([]CategoryInfo, error) {
	return listCategories(db)
}

func (s *SQLiteDB) LookupCategoryForKPI(db *sql.DB, kpiID string) (string, string, error) {
	return lookupCategoryForKPI(db, kpiID, schema.SQLiteSelectRegistryByKPI)
}

func (s *SQLiteDB) DeleteByCategory(db *sql.DB, category string) (int64, error) {
	return deleteByCategory(db, category, schema.SQLiteDeleteRegistryByCategory)
}
