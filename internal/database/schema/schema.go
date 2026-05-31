// Package schema defines the database DDL (table creation, indexes, pragmas)
// for all supported backends. Keeping DDL in a single package ensures there is
// one source of truth for the schema shape used by production code and tests.
package schema

const (
	TableClusters     = "clusters"
	TableQueryResults = "query_results"
	TableQueryErrors  = "query_errors"
	TableKPIRegistry  = "kpi_registry"
)

// KPIRegistryTable maps each KPI to its storage category and backing table.
// The schema is backend-agnostic (TEXT + TIMESTAMP work on both SQLite and PostgreSQL).
const KPIRegistryTable = `
CREATE TABLE IF NOT EXISTS kpi_registry (
    kpi_id TEXT PRIMARY KEY,
    category TEXT NOT NULL,
    table_name TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`
