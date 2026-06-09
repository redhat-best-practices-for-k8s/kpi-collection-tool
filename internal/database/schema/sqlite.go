package schema

// SQLitePragmas contains WAL mode and busy-timeout settings that improve
// write throughput and concurrency. busy_timeout must come first so that
// concurrent connections wait instead of returning SQLITE_BUSY immediately
// when the WAL journal-mode switch takes an exclusive lock.
const SQLitePragmas = `
PRAGMA busy_timeout = 5000;
PRAGMA journal_mode = WAL;
`

// SQLiteTables creates the core tables required by the KPI collector.
const SQLiteTables = `
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
    metric_labels TEXT
);

CREATE TABLE IF NOT EXISTS query_errors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kpi_id TEXT UNIQUE NOT NULL,
    errors INTEGER DEFAULT 0
);
`

// SQLiteIndexes creates all indexes for the SQLite backend.
// - dedup index: prevents duplicate data points from overlapping range query windows.
// - kpi_cluster_time: speeds up filtered queries (by KPI + time range) used by Grafana panels and CLI.
// - time_kpi_cluster: speeds up time-first scans (e.g. "all KPIs in last hour").
const SQLiteIndexes = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_query_results_dedup
ON query_results(kpi_id, cluster_id, timestamp_value, metric_labels);

CREATE INDEX IF NOT EXISTS idx_query_results_kpi_cluster_time
ON query_results(kpi_id, cluster_id, timestamp_value);

CREATE INDEX IF NOT EXISTS idx_query_results_time_kpi_cluster
ON query_results(timestamp_value, kpi_id, cluster_id);
`

// SQLiteSchema is the full DDL for initializing an SQLite database
// (tables + indexes + registry combined).
const SQLiteSchema = SQLiteTables + SQLiteIndexes + KPIRegistryTable

// SQLiteCategoryTableFmt creates a per-category table with the same column
// layout as query_results plus a dedup index. Requires three identical %s
// arguments: the table name (for CREATE TABLE, index name, and ON clause).
const SQLiteCategoryTableFmt = `
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
ON %s(kpi_id, cluster_id, timestamp_value, metric_labels);
`
