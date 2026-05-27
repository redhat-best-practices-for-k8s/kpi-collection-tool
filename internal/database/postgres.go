package database

import (
	"database/sql"
	"encoding/json"
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

// ValidateCategoryConsistency checks that no KPI in the incoming config has
// changed its category compared to what is already stored in kpi_registry.
func (p *PostgresDB) ValidateCategoryConsistency(db *sql.DB, kpis []config.Query) error {
	rows, err := db.Query(schema.PostgresSelectRegistryAll)
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
					"use a different database or delete the existing one",
				kpis[i].ID, prev, kpis[i].Category)
		}
	}

	return nil
}

// ListCategories returns all distinct categories registered in kpi_registry,
// along with the number of KPIs in each category.
func (p *PostgresDB) ListCategories(db *sql.DB) ([]CategoryInfo, error) {
	rows, err := db.Query(schema.PostgresSelectRegistryCategories)
	if err != nil {
		return nil, fmt.Errorf("query kpi_registry: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var categories []CategoryInfo
	for rows.Next() {
		var ci CategoryInfo
		if err := rows.Scan(&ci.Category, &ci.TableName, &ci.KPICount); err != nil {
			return nil, fmt.Errorf("scan kpi_registry row: %w", err)
		}
		categories = append(categories, ci)
	}

	return categories, rows.Err()
}

// LookupCategoryForKPI returns the category and table name for a KPI ID.
// Returns empty strings when the KPI has no registry entry (uncategorized).
func (p *PostgresDB) LookupCategoryForKPI(db *sql.DB, kpiID string) (category, tableName string, err error) {
	err = db.QueryRow(schema.PostgresSelectRegistryByKPI, kpiID).Scan(&category, &tableName)

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
func (p *PostgresDB) DeleteByCategory(db *sql.DB, category string) (int64, error) {
	tableName := CategoryTableName(category)

	result, err := db.Exec(fmt.Sprintf("DELETE FROM %s", tableName))
	if err != nil {
		return 0, fmt.Errorf("delete from %s: %w", tableName, err)
	}
	deleted, _ := result.RowsAffected()

	_, err = db.Exec(schema.PostgresDeleteRegistryByCategory, category)
	if err != nil {
		return deleted, fmt.Errorf("clean kpi_registry for category '%s': %w", category, err)
	}

	return deleted, nil
}

// StoreQueryResults stores the results of a Prometheus query in the database.
// When category is non-empty, writes go to the per-category table (kpi_<category>).
func (p *PostgresDB) StoreQueryResults(db *sql.DB, clusterID int64, queryID, category string, result model.Value) error {
	tableName := DefaultTableName
	if category != "" {
		name, err := p.EnsureCategoryTable(db, category, queryID)
		if err != nil {
			return fmt.Errorf("ensure category table for '%s': %w", category, err)
		}
		tableName = name
	}

	switch values := result.(type) {
	case model.Vector:
		return p.storeVectorResults(db, clusterID, queryID, tableName, values)
	case model.Matrix:
		return p.storeMatrixResults(db, clusterID, queryID, tableName, values)
	default:
		return fmt.Errorf("unsupported Prometheus result type for KPI '%s': %T", queryID, result)
	}
}

func (p *PostgresDB) storeVectorResults(db *sql.DB, clusterID int64, queryID, table string, vector model.Vector) error {
	insertSQL := fmt.Sprintf(schema.PostgresInsertResultFmt, table)

	for _, sample := range vector {
		labelsJSON, err := json.Marshal(sample.Metric)
		if err != nil {
			return err
		}

		_, err = db.Exec(insertSQL,
			queryID, float64(sample.Value), float64(sample.Timestamp)/1000, clusterID, string(labelsJSON),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *PostgresDB) storeMatrixResults(db *sql.DB, clusterID int64, queryID, table string, matrix model.Matrix) error {
	insertSQL := fmt.Sprintf(schema.PostgresInsertResultFmt, table)

	for _, stream := range matrix {
		labelsJSON, err := json.Marshal(stream.Metric)
		if err != nil {
			return err
		}

		for _, samplePair := range stream.Values {
			_, execErr := db.Exec(insertSQL,
				queryID, float64(samplePair.Value), float64(samplePair.Timestamp)/1000, clusterID, string(labelsJSON),
			)
			if execErr != nil {
				return execErr
			}
		}
	}

	return nil
}
