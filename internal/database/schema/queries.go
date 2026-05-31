package schema

// ---------------------------------------------------------------------------
// Shared queries — identical for both SQLite and PostgreSQL (no placeholders).
// ---------------------------------------------------------------------------

const SelectRegistryAll = `SELECT kpi_id, category FROM kpi_registry`

const SelectRegistryCategories = `
SELECT category, table_name, COUNT(*) as kpi_count
FROM kpi_registry
GROUP BY category, table_name
ORDER BY category`

// ---------------------------------------------------------------------------
// SQLite DML — uses ? placeholders.
// ---------------------------------------------------------------------------

// SQLiteInsertResultFmt inserts a metric row into any table (query_results or
// a per-category table). Requires one %s argument: the target table name.
const SQLiteInsertResultFmt = `
INSERT INTO %s (kpi_id, metric_value, timestamp_value, cluster_id, metric_labels)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(kpi_id, cluster_id, timestamp_value, metric_labels) DO NOTHING`

const SQLiteUpsertQueryError = `
INSERT INTO query_errors (kpi_id, errors) VALUES (?, 1)
ON CONFLICT(kpi_id) DO UPDATE SET errors = errors + 1`

const SQLiteSelectErrorCount = `SELECT errors FROM query_errors WHERE kpi_id = ?`

const SQLiteSelectClusterByName = `SELECT id FROM clusters WHERE cluster_name = ?`

const SQLiteUpdateClusterType = `UPDATE clusters SET cluster_type = ? WHERE id = ?`

const SQLiteInsertCluster = `INSERT INTO clusters (cluster_name, cluster_type) VALUES (?, ?)`

const SQLiteUpsertRegistry = `
INSERT INTO kpi_registry (kpi_id, category, table_name) VALUES (?, ?, ?)
ON CONFLICT(kpi_id) DO NOTHING`

const SQLiteSelectRegistryByKPI = `SELECT category, table_name FROM kpi_registry WHERE kpi_id = ?`

const SQLiteDeleteRegistryByCategory = `DELETE FROM kpi_registry WHERE category = ?`

// ---------------------------------------------------------------------------
// PostgreSQL DML — uses $N placeholders.
// ---------------------------------------------------------------------------

// PostgresInsertResultFmt inserts a metric row into any table (query_results or
// a per-category table). Requires one %s argument: the target table name.
const PostgresInsertResultFmt = `
INSERT INTO %s (kpi_id, metric_value, timestamp_value, cluster_id, metric_labels)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (kpi_id, cluster_id, timestamp_value, metric_labels) DO NOTHING`

const PostgresUpsertQueryError = `
INSERT INTO query_errors (kpi_id, errors) VALUES ($1, 1)
ON CONFLICT(kpi_id) DO UPDATE SET errors = query_errors.errors + 1`

const PostgresSelectErrorCount = `SELECT errors FROM query_errors WHERE kpi_id = $1`

const PostgresSelectClusterByName = `SELECT id FROM clusters WHERE cluster_name = $1`

const PostgresUpdateClusterType = `UPDATE clusters SET cluster_type = $1 WHERE id = $2`

const PostgresUpsertCluster = `
INSERT INTO clusters (cluster_name, cluster_type) VALUES ($1, $2)
ON CONFLICT (cluster_name) DO UPDATE SET cluster_type = EXCLUDED.cluster_type
RETURNING id`

const PostgresUpsertRegistry = `
INSERT INTO kpi_registry (kpi_id, category, table_name) VALUES ($1, $2, $3)
ON CONFLICT(kpi_id) DO NOTHING`

const PostgresSelectRegistryByKPI = `SELECT category, table_name FROM kpi_registry WHERE kpi_id = $1`

const PostgresDeleteRegistryByCategory = `DELETE FROM kpi_registry WHERE category = $1`
