package schema

// PostgreSQL DML queries use $N placeholders.

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

// --- kpi_registry queries ---

const PostgresUpsertRegistry = `
INSERT INTO kpi_registry (kpi_id, category, table_name) VALUES ($1, $2, $3)
ON CONFLICT(kpi_id) DO NOTHING`

const PostgresSelectRegistryAll = `SELECT kpi_id, category FROM kpi_registry`

const PostgresSelectRegistryCategories = `
SELECT category, table_name, COUNT(*) as kpi_count
FROM kpi_registry
GROUP BY category, table_name
ORDER BY category`

const PostgresSelectRegistryByKPI = `SELECT category, table_name FROM kpi_registry WHERE kpi_id = $1`

const PostgresDeleteRegistryByCategory = `DELETE FROM kpi_registry WHERE category = $1`
