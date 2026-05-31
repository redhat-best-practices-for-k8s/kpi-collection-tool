package database

import (
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/lib/pq"
	"github.com/prometheus/common/model"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/database/schema"
)

// PostgresDB implements the Database interface for PostgreSQL
type PostgresDB struct {
	ConnectionURL string
	knownTables   sync.Map
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

// GetOrCreateCluster gets existing cluster ID or creates a new cluster record.
// Uses PostgreSQL's RETURNING clause to obtain the new row ID.
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

// EnsureCategoryTable lazily creates the per-category table and registers the
// KPI->category mapping. The DDL is idempotent and only executed once per
// process lifetime thanks to the in-memory knownTables cache.
func (p *PostgresDB) EnsureCategoryTable(db *sql.DB, category, kpiID string) (string, error) {
	tableName := CategoryTableName(category)

	if _, ok := p.knownTables.Load(tableName); !ok {
		ddl := fmt.Sprintf(schema.PostgresCategoryTableFmt, tableName, tableName, tableName)

		if _, err := db.Exec(ddl); err != nil {
			return "", fmt.Errorf("create table %s: %w", tableName, err)
		}

		p.knownTables.Store(tableName, true)
	}

	_, err := db.Exec(schema.PostgresUpsertRegistry, kpiID, category, tableName)
	if err != nil {
		return "", fmt.Errorf("register kpi '%s' in kpi_registry: %w", kpiID, err)
	}

	return tableName, nil
}

// --- Thin wrappers delegating to shared free functions ---

func (p *PostgresDB) IncrementQueryError(db *sql.DB, kpiID string) error {
	return incrementQueryError(db, kpiID, schema.PostgresUpsertQueryError)
}

func (p *PostgresDB) GetQueryErrorCount(db *sql.DB, kpiID string) (int, error) {
	return getQueryErrorCount(db, kpiID, schema.PostgresSelectErrorCount)
}

func (p *PostgresDB) StoreQueryResults(db *sql.DB, clusterID int64, queryID, category string, result model.Value) error {
	return storeQueryResults(db, clusterID, queryID, category, result, p.EnsureCategoryTable, schema.PostgresInsertResultFmt)
}

func (p *PostgresDB) ValidateCategoryConsistency(db *sql.DB, kpis []config.Query) error {
	return validateCategoryConsistency(db, kpis, "use a different database or delete the existing one")
}

func (p *PostgresDB) ListCategories(db *sql.DB) ([]CategoryInfo, error) {
	return listCategories(db)
}

func (p *PostgresDB) LookupCategoryForKPI(db *sql.DB, kpiID string) (category, tableName string, err error) {
	return lookupCategoryForKPI(db, kpiID, schema.PostgresSelectRegistryByKPI)
}

func (p *PostgresDB) DeleteByCategory(db *sql.DB, category string) (int64, error) {
	return deleteByCategory(db, category, schema.PostgresDeleteRegistryByCategory)
}
