// Package schema defines the database DDL (table creation, indexes, pragmas)
// for all supported backends. Keeping DDL in a single package ensures there is
// one source of truth for the schema shape used by production code and tests.
package schema

const (
	TableClusters     = "clusters"
	TableQueryResults = "query_results"
	TableQueryErrors  = "query_errors"
)
