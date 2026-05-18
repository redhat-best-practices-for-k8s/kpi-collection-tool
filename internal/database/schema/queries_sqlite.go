package schema

// SQLite DML queries use ? placeholders.

const SQLiteInsertResult = `
INSERT INTO query_results
(kpi_id, metric_value, timestamp_value, cluster_id, metric_labels)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(kpi_id, cluster_id, timestamp_value, metric_labels) DO NOTHING`

const SQLiteUpsertQueryError = `
INSERT INTO query_errors (kpi_id, errors) VALUES (?, 1)
ON CONFLICT(kpi_id) DO UPDATE SET errors = errors + 1`

const SQLiteSelectErrorCount = `SELECT errors FROM query_errors WHERE kpi_id = ?`

const SQLiteSelectClusterByName = `SELECT id FROM clusters WHERE cluster_name = ?`

const SQLiteUpdateClusterType = `UPDATE clusters SET cluster_type = ? WHERE id = ?`

const SQLiteInsertCluster = `INSERT INTO clusters (cluster_name, cluster_type) VALUES (?, ?)`
