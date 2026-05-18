package schema

// PostgreSQL DML queries use $N placeholders.

const PostgresInsertResult = `
INSERT INTO query_results
(kpi_id, metric_value, timestamp_value, cluster_id, metric_labels)
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
