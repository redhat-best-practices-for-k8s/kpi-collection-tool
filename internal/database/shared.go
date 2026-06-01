package database

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/prometheus/common/model"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/database/schema"
)

// CategoryTableName returns the physical table name for a given sanitised category.
func CategoryTableName(category string) string {
	return "kpi_" + category
}

// ---------------------------------------------------------------------------
// Shared free functions — called by both SQLiteDB and PostgresDB wrappers.
// ---------------------------------------------------------------------------

func incrementQueryError(db *sql.DB, kpiID, upsertQuery string) error {
	_, err := db.Exec(upsertQuery, kpiID)
	return err
}

func getQueryErrorCount(db *sql.DB, kpiID, selectQuery string) (int, error) {
	var count int
	err := db.QueryRow(selectQuery, kpiID).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return count, err
}

// storeQueryResults routes Vector/Matrix results to the correct storage
// function, creating the category table on-the-fly when needed.
func storeQueryResults(
	db *sql.DB, clusterID int64, queryID, category string, result model.Value,
	ensureCategory func(*sql.DB, string, string) (string, error),
	insertFmt string,
) error {
	tableName := DefaultTableName
	if category != "" {
		name, err := ensureCategory(db, category, queryID)
		if err != nil {
			return fmt.Errorf("ensure category table for '%s': %w", category, err)
		}
		tableName = name
	}

	switch values := result.(type) {
	case model.Vector:
		return storeVectorResults(db, clusterID, queryID, tableName, insertFmt, values)
	case model.Matrix:
		return storeMatrixResults(db, clusterID, queryID, tableName, insertFmt, values)
	default:
		return fmt.Errorf("unsupported Prometheus result type for KPI '%s': %T", queryID, result)
	}
}

func storeVectorResults(db *sql.DB, clusterID int64, queryID, table, insertFmt string, vector model.Vector) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(fmt.Sprintf(insertFmt, table))
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, sample := range vector {
		labelsJSON, err := json.Marshal(sample.Metric)
		if err != nil {
			return err
		}
		if _, err = stmt.Exec(
			queryID,
			float64(sample.Value),
			float64(sample.Timestamp)/1000,
			clusterID,
			string(labelsJSON),
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func storeMatrixResults(db *sql.DB, clusterID int64, queryID, table, insertFmt string, matrix model.Matrix) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(fmt.Sprintf(insertFmt, table))
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, stream := range matrix {
		labelsJSON, err := json.Marshal(stream.Metric)
		if err != nil {
			return err
		}

		for _, samplePair := range stream.Values {
			if _, err = stmt.Exec(
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

	return tx.Commit()
}

// validateCategoryConsistency checks that no KPI in the incoming config has
// changed its category compared to what is already stored in kpi_registry.
// errHint provides a backend-specific recovery suggestion in the error message.
func validateCategoryConsistency(db *sql.DB, kpis []config.Query, errHint string) error {
	rows, err := db.Query(schema.SelectRegistryAll)
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
				"KPI '%s' category changed from %q to %q — %s",
				kpis[i].ID, prev, kpis[i].Category, errHint)
		}
	}

	return nil
}

func listCategories(db *sql.DB) ([]CategoryInfo, error) {
	rows, err := db.Query(schema.SelectRegistryCategories)
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

func lookupCategoryForKPI(db *sql.DB, kpiID, selectQuery string) (string, string, error) {
	var category, tableName string
	err := db.QueryRow(selectQuery, kpiID).Scan(&category, &tableName)

	if err == sql.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("lookup kpi_registry for '%s': %w", kpiID, err)
	}

	return category, tableName, nil
}

func deleteByCategory(db *sql.DB, category, deleteRegistryQuery string) (int64, error) {
	tableName := CategoryTableName(category)

	result, err := db.Exec(fmt.Sprintf("DELETE FROM %s", tableName))
	if err != nil {
		return 0, fmt.Errorf("delete from %s: %w", tableName, err)
	}
	deleted, _ := result.RowsAffected()

	_, err = db.Exec(deleteRegistryQuery, category)
	if err != nil {
		return deleted, fmt.Errorf("clean kpi_registry for category '%s': %w", category, err)
	}

	return deleted, nil
}
