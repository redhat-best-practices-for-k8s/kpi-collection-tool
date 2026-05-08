package commands

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/database"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/output"

	"github.com/spf13/cobra"
)

// kpiQueryFlags holds the flags for the 'show kpis' command
var kpiQueryFlags struct {
	kpiName      string
	clusterName  string
	labelsFilter string
	since        string
	until        string
	limit        int
	sort         string
	noTruncate   bool
	outputFormat string
}

// clusterQueryFlags holds the flag for the 'show clusters' command
var clusterQueryFlags struct {
	clusterName string
}

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Query and display data from the database",
	Long:  `Query and display KPI metrics, clusters, or errors from the database.`,
}

var showKPIsCmd = &cobra.Command{
	Use:   "kpis",
	Short: "Show KPI metrics",
	Long: `Query and display KPI metrics with optional filtering by name, cluster, labels, and time range.

The results can be displayed in table, JSON, or CSV format.`,
	Example: `  # Show all metrics for a KPI
  kpi-collector db show kpis --name="cpu-system"
  
  # Filter by cluster
  kpi-collector db show kpis --name="cpu-system" --cluster-name="mycluster1"
  
  # Filter by labels (exact match)
  kpi-collector db show kpis --name="cpu-system" \
    --labels-filter='id=/system.slice/systemd-logind.service'
  
  # Time-based filtering
  kpi-collector db show kpis --name="cpu-system" --since="2h" --until="1h"

  # Time-based filtering with RFC3339 timestamps
  kpi-collector db show kpis --name="cpu-system" --since="2026-04-07T12:24:25Z" --until="2026-04-08T22:34:25Z"
  
  # Limit results and sort
  kpi-collector db show kpis --name="cpu-pods" --limit=100 --sort="desc"
  
  # Output as JSON
  kpi-collector db show kpis --name="cpu-system" -o json
  
  # Export to CSV file
  kpi-collector db show kpis --name="cpu-system" -o csv > metrics.csv`,
	RunE: runShowKPIs,
}

var showClustersCmd = &cobra.Command{
	Use:   "clusters",
	Short: "List all monitored clusters",
	Long:  `Display all clusters that have been monitored, with their creation dates and metric counts.`,
	Example: `  # List all clusters
  kpi-collector db show clusters
  
  # Filter by specific cluster
  kpi-collector db show clusters --name="mycluster1"`,
	RunE: runShowClusters,
}

var showErrorsCmd = &cobra.Command{
	Use:   "errors",
	Short: "Show query error counts",
	Long: `Display KPI queries that have encountered errors during collection.
Shows the error count per KPI — not the error details. To see the actual
error messages, check the log file in the artifacts directory.`,
	Example: `  # List all query errors
  kpi-collector db show errors`,
	RunE: runShowErrors,
}

func init() {
	dbCmd.AddCommand(showCmd)
	showCmd.AddCommand(showKPIsCmd)
	showCmd.AddCommand(showClustersCmd)
	showCmd.AddCommand(showErrorsCmd)

	// Flags for 'show kpis'
	showKPIsCmd.Flags().StringVar(&kpiQueryFlags.kpiName, "name", "",
		"KPI name to filter by")
	showKPIsCmd.Flags().StringVar(&kpiQueryFlags.clusterName, "cluster-name", "",
		"cluster name to filter by")
	showKPIsCmd.Flags().StringVar(&kpiQueryFlags.labelsFilter, "labels-filter", "",
		"label filters in format 'key=value,key2=value2'")
	showKPIsCmd.Flags().StringVar(&kpiQueryFlags.since, "since", "",
		"show metrics since sample timestamp (Go duration or RFC3339, e.g. '2h' or '2026-04-07T12:24:25Z')")
	showKPIsCmd.Flags().StringVar(&kpiQueryFlags.until, "until", "",
		"show metrics until sample timestamp (Go duration or RFC3339, e.g. '1h' or '2026-04-08T22:34:25Z')")
	showKPIsCmd.Flags().IntVar(&kpiQueryFlags.limit, "limit", 0,
		"limit number of results (0 = no limit)")
	showKPIsCmd.Flags().StringVar(&kpiQueryFlags.sort, "sort", "asc",
		"sort order by metric timestamp: asc or desc")
	showKPIsCmd.Flags().BoolVar(&kpiQueryFlags.noTruncate, "no-truncate", false,
		"show full labels without truncation")
	showKPIsCmd.Flags().StringVarP(&kpiQueryFlags.outputFormat, "output", "o", "table",
		"output format: table, json, or csv")

	// Flags for 'show clusters'
	showClustersCmd.Flags().StringVar(&clusterQueryFlags.clusterName, "name", "",
		"specific cluster name to filter by")
}

func runShowKPIs(cmd *cobra.Command, args []string) error {
	// Parse output format first (fail fast if invalid)
	format, err := output.ParseFormat(kpiQueryFlags.outputFormat)
	if err != nil {
		return err
	}

	db, dbImpl, err := connectToDB()
	if err != nil {
		return fmt.Errorf("failed to connect to DB: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Parse time filters using a single reference time to avoid drift.
	sinceTime, untilTime, err := parseKPIQueryTimeWindow(kpiQueryFlags.since, kpiQueryFlags.until, time.Now())
	if err != nil {
		return err
	}

	// Parse label filters
	labelFilters := make(map[string]string)
	if kpiQueryFlags.labelsFilter != "" {
		labelFilters, err = parseLabelFilters(kpiQueryFlags.labelsFilter)
		if err != nil {
			return fmt.Errorf("invalid --labels-filter: %w", err)
		}
	}

	// Build query parameters
	params := KPIQueryParams{
		KPIName:      kpiQueryFlags.kpiName,
		ClusterName:  kpiQueryFlags.clusterName,
		LabelFilters: labelFilters,
		Since:        sinceTime,
		Until:        untilTime,
		Limit:        kpiQueryFlags.limit,
		Sort:         kpiQueryFlags.sort,
	}

	// Query KPIs
	results, err := queryKPIs(db, dbImpl, params)
	if err != nil {
		return fmt.Errorf("failed to query KPIs: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	// Convert to output records
	records := convertToKPIRecords(results)

	// Print using the output package
	printer := output.NewPrinter(format).WithNoTruncate(kpiQueryFlags.noTruncate)
	return printer.PrintKPIs(records)
}

func runShowClusters(cmd *cobra.Command, args []string) error {
	db, dbImpl, err := connectToDB()
	if err != nil {
		return fmt.Errorf("failed to connect to DB: %w", err)
	}
	defer func() { _ = db.Close() }()

	clusters, err := listClusters(db, dbImpl, clusterQueryFlags.clusterName)
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	if len(clusters) == 0 {
		fmt.Println("No clusters found.")
		return nil
	}

	// Convert to output records
	records := make([]output.ClusterRecord, len(clusters))
	for i, c := range clusters {
		records[i] = output.ClusterRecord{
			ID:           c.ID,
			Name:         c.Name,
			CreatedAt:    c.CreatedAt,
			TotalMetrics: c.TotalMetrics,
		}
	}

	output.PrintClustersTable(records)
	return nil
}

func runShowErrors(cmd *cobra.Command, args []string) error {
	db, _, err := connectToDB()
	if err != nil {
		return fmt.Errorf("failed to connect to DB: %w", err)
	}
	defer func() { _ = db.Close() }()

	errors, err := listErrors(db)
	if err != nil {
		return fmt.Errorf("failed to list errors: %w", err)
	}

	if len(errors) == 0 {
		fmt.Println("No errors found.")
		return nil
	}

	// Convert to output records
	records := make([]output.ErrorRecord, len(errors))
	for i, e := range errors {
		records[i] = output.ErrorRecord{
			KPIID:      e.KPIID,
			ErrorCount: e.ErrorCount,
		}
	}

	output.PrintErrorsTable(records)
	return nil
}

func parseKPIQueryTimeWindow(sinceInput, untilInput string, now time.Time) (*time.Time, *time.Time, error) {
	var sinceTime, untilTime *time.Time

	if strings.TrimSpace(sinceInput) != "" {
		t, err := parseTimeFilter(sinceInput, now)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid --since value: %w", err)
		}
		sinceTime = &t
	}

	if strings.TrimSpace(untilInput) != "" {
		t, err := parseTimeFilter(untilInput, now)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid --until value: %w", err)
		}
		untilTime = &t
	}

	if sinceTime != nil && untilTime != nil && !sinceTime.Before(*untilTime) {
		return nil, nil, fmt.Errorf("invalid time window: --since must resolve before --until (since: %s, until: %s)",
			sinceTime.Format(time.RFC3339), untilTime.Format(time.RFC3339))
	}

	return sinceTime, untilTime, nil
}

func parseTimeFilter(timeStr string, now time.Time) (time.Time, error) {
	trimmed := strings.TrimSpace(timeStr)

	if duration, err := time.ParseDuration(trimmed); err == nil {
		if duration <= 0 {
			return time.Time{}, fmt.Errorf("must be > 0 when specified as a duration")
		}
		return now.Add(-duration), nil
	}

	if absolute, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return absolute, nil
	}

	return time.Time{}, fmt.Errorf("must be a Go duration (e.g. \"2h\") or RFC3339 format (e.g. \"2026-04-07T12:24:25Z\")")
}

func parseLabelFilters(filterStr string) (map[string]string, error) {
	filters := make(map[string]string)
	pairs := strings.Split(filterStr, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid label filter format: %s (use key=value)", pair)
		}
		filters[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return filters, nil
}

type KPIResult struct {
	ID             int64
	KPIName        string
	ClusterName    string
	MetricValue    float64
	TimestampValue float64
	ExecutionTime  time.Time
	MetricLabels   string
}

type KPIQueryParams struct {
	KPIName      string
	ClusterName  string
	LabelFilters map[string]string
	Since        *time.Time
	Until        *time.Time
	Limit        int
	Sort         string
}

func queryKPIs(db *sql.DB, dbImpl database.Database, params KPIQueryParams) ([]KPIResult, error) {
	query := `
		SELECT qr.id, qr.kpi_id, c.cluster_name, qr.metric_value, 
		       qr.timestamp_value, qr.execution_time, qr.metric_labels
		FROM query_results qr
		JOIN clusters c ON qr.cluster_id = c.id
		WHERE 1=1
	`

	args := []interface{}{}
	argIndex := 1

	if params.KPIName != "" {
		query += fmt.Sprintf(" AND qr.kpi_id = $%d", argIndex)
		args = append(args, params.KPIName)
		argIndex++
	}

	if params.ClusterName != "" {
		query += fmt.Sprintf(" AND c.cluster_name = $%d", argIndex)
		args = append(args, params.ClusterName)
		argIndex++
	}

	if params.Since != nil {
		query += fmt.Sprintf(" AND qr.timestamp_value >= $%d", argIndex)
		// CLI time filters intentionally use second precision (0 milliseconds).
		// We truncate to whole seconds, then convert to Unix epoch seconds for
		// comparison against query_results.timestamp_value.
		args = append(args, float64(params.Since.Truncate(time.Second).Unix()))
		argIndex++
	}

	if params.Until != nil {
		query += fmt.Sprintf(" AND qr.timestamp_value <= $%d", argIndex)
		args = append(args, float64(params.Until.Truncate(time.Second).Unix()))
		argIndex++
	}

	if params.Sort == "desc" {
		query += " ORDER BY qr.timestamp_value DESC"
	} else {
		query += " ORDER BY qr.timestamp_value ASC"
	}

	if params.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, params.Limit)
	}

	// Convert placeholders for SQLite
	if _, ok := dbImpl.(*database.SQLiteDB); ok {
		query = convertPostgresToSQLitePlaceholders(query)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []KPIResult
	for rows.Next() {
		var r KPIResult
		err := rows.Scan(&r.ID, &r.KPIName, &r.ClusterName, &r.MetricValue,
			&r.TimestampValue, &r.ExecutionTime, &r.MetricLabels)
		if err != nil {
			return nil, err
		}

		// Apply label filters
		if len(params.LabelFilters) > 0 {
			if !matchesLabelFilters(r.MetricLabels, params.LabelFilters) {
				continue
			}
		}

		results = append(results, r)
	}

	return results, rows.Err()
}

func matchesLabelFilters(labelsJSON string, filters map[string]string) bool {
	var labels map[string]string
	if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
		return false
	}

	for key, value := range filters {
		labelValue, exists := labels[key]
		if !exists || labelValue != value {
			return false
		}
	}
	return true
}

type ClusterInfo struct {
	ID           int64
	Name         string
	CreatedAt    time.Time
	TotalMetrics int64
}

func listClusters(db *sql.DB, dbImpl database.Database, clusterName string) ([]ClusterInfo, error) {
	query := `
		SELECT c.id, c.cluster_name, c.created_at, COUNT(qr.id) as total_metrics
		FROM clusters c
		LEFT JOIN query_results qr ON c.id = qr.cluster_id
	`
	args := []interface{}{}

	if clusterName != "" {
		query += " WHERE c.cluster_name = $1"
		args = append(args, clusterName)
	}

	query += " GROUP BY c.id, c.cluster_name, c.created_at ORDER BY c.created_at DESC"

	if _, ok := dbImpl.(*database.SQLiteDB); ok {
		query = convertPostgresToSQLitePlaceholders(query)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var clusters []ClusterInfo
	for rows.Next() {
		var c ClusterInfo
		err := rows.Scan(&c.ID, &c.Name, &c.CreatedAt, &c.TotalMetrics)
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, c)
	}

	return clusters, rows.Err()
}

type ErrorInfo struct {
	KPIID      string
	ErrorCount int
}

func listErrors(db *sql.DB) ([]ErrorInfo, error) {
	query := "SELECT kpi_id, errors FROM query_errors WHERE errors > 0 ORDER BY errors DESC"

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var errors []ErrorInfo
	for rows.Next() {
		var e ErrorInfo
		err := rows.Scan(&e.KPIID, &e.ErrorCount)
		if err != nil {
			return nil, err
		}
		errors = append(errors, e)
	}

	return errors, rows.Err()
}

// convertToKPIRecords converts internal KPIResult to output.KPIRecord
func convertToKPIRecords(results []KPIResult) []output.KPIRecord {
	records := make([]output.KPIRecord, len(results))
	for i, r := range results {
		// Parse labels JSON into map
		var labels map[string]string
		_ = json.Unmarshal([]byte(r.MetricLabels), &labels)

		records[i] = output.KPIRecord{
			ID:            r.ID,
			KPIName:       r.KPIName,
			Cluster:       r.ClusterName,
			Value:         r.MetricValue,
			Timestamp:     r.TimestampValue,
			ExecutionTime: r.ExecutionTime,
			Labels:        labels,
			LabelsRaw:     r.MetricLabels,
		}
	}
	return records
}

func convertPostgresToSQLitePlaceholders(query string) string {
	result := query
	for i := 20; i >= 1; i-- {
		result = strings.ReplaceAll(result, fmt.Sprintf("$%d", i), "?")
	}
	return result
}
