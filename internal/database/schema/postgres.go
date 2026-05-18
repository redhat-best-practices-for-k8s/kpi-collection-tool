package schema

// PostgresTables creates the core tables required by the KPI collector
// using PostgreSQL-specific types (SERIAL, DOUBLE PRECISION, JSONB).
const PostgresTables = `
CREATE TABLE IF NOT EXISTS clusters (
    id SERIAL PRIMARY KEY,
    cluster_name TEXT UNIQUE NOT NULL,
    cluster_type TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS query_results (
    id SERIAL PRIMARY KEY,
    kpi_id TEXT NOT NULL,
    metric_value DOUBLE PRECISION,
    timestamp_value DOUBLE PRECISION,
    cluster_id INTEGER NOT NULL REFERENCES clusters(id),
    execution_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    metric_labels JSONB
);

CREATE TABLE IF NOT EXISTS query_errors (
    id SERIAL PRIMARY KEY,
    kpi_id TEXT UNIQUE NOT NULL,
    errors INTEGER DEFAULT 0
);
`

// PostgresIndexes creates all indexes for the PostgreSQL backend.
// - dedup index: prevents duplicate data points from overlapping range query windows.
// - cluster_id, kpi_id, created_at: single-column indexes for common filters.
// - GIN on metric_labels: enables fast JSONB containment queries.
// - kpi_cluster_time / time_kpi_cluster: composite indexes for Grafana panel queries.
const PostgresIndexes = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_query_results_dedup
ON query_results(kpi_id, cluster_id, timestamp_value, metric_labels);

CREATE INDEX IF NOT EXISTS idx_query_results_cluster_id ON query_results(cluster_id);
CREATE INDEX IF NOT EXISTS idx_query_results_kpi_id ON query_results(kpi_id);
CREATE INDEX IF NOT EXISTS idx_query_results_created_at ON query_results(created_at);
CREATE INDEX IF NOT EXISTS idx_query_results_labels ON query_results USING GIN(metric_labels);

CREATE INDEX IF NOT EXISTS idx_query_results_kpi_cluster_time
ON query_results(kpi_id, cluster_id, timestamp_value);

CREATE INDEX IF NOT EXISTS idx_query_results_time_kpi_cluster
ON query_results(timestamp_value, kpi_id, cluster_id);
`

// PostgresSchema is the full DDL for initializing a PostgreSQL database
// (tables + indexes combined).
const PostgresSchema = PostgresTables + PostgresIndexes
