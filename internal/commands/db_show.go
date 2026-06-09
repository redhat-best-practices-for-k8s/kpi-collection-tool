package commands

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/database"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/output"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// kpiQueryFlags holds the flags for the 'show kpis' command
var kpiQueryFlags struct {
	kpiName      string
	category     string
	clusterName  string
	labelsFilter string
	since        string
	until        string
	limit        int
	sort         string
	noTruncate   bool
	showExecTime bool
	outputFormat string
	chartWidth   int
	chartHeight  int
	interactive  bool
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
	Long: `Query and display KPI metrics with optional filtering by name, category, cluster, labels, and time range.

The results can be displayed in table, JSON, or CSV format.`,
	Example: `  # Show all metrics for a KPI
  kpi-collector db show kpis --name="cpu-system"
  
  # Show all metrics in a category
  kpi-collector db show kpis --category=cpu
  
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
  kpi-collector db show kpis --name="cpu-system" -o csv > metrics.csv

  # Plot an ASCII chart of metric values over the last 24 hours
  kpi-collector db show kpis --name="cpu-system" --since="24h" -o chart`,
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

var showCategoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "List all KPI categories",
	Long: `Display all KPI categories registered in the database.
Shows the category name, backing table, and number of KPIs per category.`,
	Example: `  # List all categories
  kpi-collector db show categories`,
	RunE: runShowCategories,
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
	showCmd.AddCommand(showCategoriesCmd)
	showCmd.AddCommand(showErrorsCmd)

	// Flags for 'show kpis'
	showKPIsCmd.Flags().StringVar(&kpiQueryFlags.kpiName, "name", "",
		"KPI name to filter by")
	showKPIsCmd.Flags().StringVar(&kpiQueryFlags.category, "category", "",
		"category to filter by (e.g. cpu, memory, network)")
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
		"sort order by metric timestamp (UTC): asc or desc")
	showKPIsCmd.Flags().BoolVar(&kpiQueryFlags.noTruncate, "no-truncate", false,
		"show full labels without truncation")
	showKPIsCmd.Flags().BoolVar(&kpiQueryFlags.showExecTime, "show-exec-time", false,
		"include execution time (when the metric was collected) in the output")
	showKPIsCmd.Flags().StringVarP(&kpiQueryFlags.outputFormat, "output", "o", "table",
		"output format: table, json, csv, or chart")
	showKPIsCmd.Flags().IntVar(&kpiQueryFlags.chartWidth, "chart-width", 0,
		fmt.Sprintf("total chart width in columns (%d-%d, default: terminal width or %d for non-TTY; requires -o chart)",
			output.MinChartWidth, output.MaxChartDimension, output.DefaultChartWidth))
	showKPIsCmd.Flags().IntVar(&kpiQueryFlags.chartHeight, "chart-height", 0,
		fmt.Sprintf("total chart height in rows (%d-%d, default: terminal height or %d for non-TTY; requires -o chart)",
			output.MinChartHeight, output.MaxChartDimension, output.DefaultChartHeight))
	showKPIsCmd.Flags().BoolVar(&kpiQueryFlags.interactive, "interactive", false,
		"interactive full-screen chart with keyboard navigation (requires -o chart and a TTY)")

	// Flags for 'show clusters'
	showClustersCmd.Flags().StringVar(&clusterQueryFlags.clusterName, "name", "",
		"specific cluster name to filter by")
}

// validateShowKPIsCLIFlags validates the CLI flags for the 'show kpis' command. For
// convenience, it returns the output format as well as any errors encountered.
func validateShowKPIsCLIFlags(cmd *cobra.Command) (output.Format, error) {
	if kpiQueryFlags.kpiName != "" && kpiQueryFlags.category != "" {
		return "", fmt.Errorf("--name and --category cannot be used together: " +
			"--name looks up a specific KPI (auto-discovers its table), " +
			"--category queries all KPIs in a category")
	}

	format, err := output.ParseFormat(kpiQueryFlags.outputFormat)
	if err != nil {
		return "", err
	}

	if format == output.FormatChart && kpiQueryFlags.kpiName == "" {
		return "", fmt.Errorf("-o chart requires --name to be set")
	}

	if err := validateChartDimensionFlags(cmd, format); err != nil {
		return "", err
	}

	if err := validateInteractiveFlag(cmd, format); err != nil {
		return "", err
	}

	return format, nil
}

func runShowKPIs(cmd *cobra.Command, args []string) error {
	outputFormat, err := validateShowKPIsCLIFlags(cmd)
	if err != nil {
		return fmt.Errorf("failed to validate CLI flags: %w", err)
	}

	db, dbImpl, err := connectToDB()
	if err != nil {
		return fmt.Errorf("failed to connect to DB: %w", err)
	}
	defer func() { _ = db.Close() }()

	if kpiQueryFlags.category != "" {
		if err := validateCategory(db, dbImpl, kpiQueryFlags.category); err != nil {
			return fmt.Errorf("failed to validate category: %w", err)
		}
	}

	// Parse time filters using a single reference time to avoid drift.
	sinceTime, untilTime, err := parseKPIQueryTimeWindow(kpiQueryFlags.since, kpiQueryFlags.until, time.Now())
	if err != nil {
		return fmt.Errorf("failed to parse time filters: %w", err)
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
		Category:     kpiQueryFlags.category,
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
		switch outputFormat {
		case output.FormatJSON:
			fmt.Println("[]")
		case output.FormatCSV:
			fmt.Println("id,kpi_name,category,cluster,value,timestamp,labels")
		default:
			fmt.Println("No results found.")
		}
		return nil
	}

	// Convert to output records
	records := convertToKPIRecords(results)

	if outputFormat == output.FormatChart {
		if kpiQueryFlags.interactive {
			return output.RunInteractiveChart(records, kpiQueryFlags.kpiName)
		}
		output.PrintChart(records, kpiQueryFlags.kpiName, kpiQueryFlags.chartWidth, kpiQueryFlags.chartHeight)
		return nil
	}

	// Print using the output package
	printer := output.NewPrinter(outputFormat).
		WithNoTruncate(kpiQueryFlags.noTruncate).
		WithShowExecTime(kpiQueryFlags.showExecTime)
	return printer.PrintKPIs(records)
}

func validateChartDimensionFlags(cmd *cobra.Command, format output.Format) error {
	widthSet := cmd.Flags().Changed("chart-width")
	heightSet := cmd.Flags().Changed("chart-height")

	if (widthSet || heightSet) && format != output.FormatChart {
		return fmt.Errorf("--chart-width and --chart-height require -o chart")
	}

	if widthSet && (kpiQueryFlags.chartWidth < output.MinChartWidth || kpiQueryFlags.chartWidth > output.MaxChartDimension) {
		return fmt.Errorf("--chart-width must be between %d and %d (got %d)",
			output.MinChartWidth, output.MaxChartDimension, kpiQueryFlags.chartWidth)
	}

	if heightSet && (kpiQueryFlags.chartHeight < output.MinChartHeight || kpiQueryFlags.chartHeight > output.MaxChartDimension) {
		return fmt.Errorf("--chart-height must be between %d and %d (got %d)",
			output.MinChartHeight, output.MaxChartDimension, kpiQueryFlags.chartHeight)
	}

	if err := validateChartDimensionsFitTerminal(widthSet, heightSet); err != nil {
		return err
	}

	return nil
}

// validateChartDimensionsFitTerminal checks that user-supplied chart
// dimensions don't exceed the actual terminal size. Only applies when
// stdout is a TTY and at least one dimension override is set.
func validateChartDimensionsFitTerminal(widthSet, heightSet bool) error {
	if !widthSet && !heightSet {
		return nil
	}

	fd := int(os.Stdout.Fd())
	if !term.IsTerminal(fd) {
		return nil
	}

	ttyWidth, ttyHeight, err := term.GetSize(fd)
	if err != nil {
		return nil
	}

	if widthSet && kpiQueryFlags.chartWidth > ttyWidth {
		return fmt.Errorf("--chart-width (%d) exceeds terminal width (%d)", kpiQueryFlags.chartWidth, ttyWidth)
	}

	if heightSet && kpiQueryFlags.chartHeight > ttyHeight {
		return fmt.Errorf("--chart-height (%d) exceeds terminal height (%d)", kpiQueryFlags.chartHeight, ttyHeight)
	}

	return nil
}

func validateInteractiveFlag(cmd *cobra.Command, format output.Format) error {
	if !kpiQueryFlags.interactive {
		return nil
	}

	if format != output.FormatChart {
		return fmt.Errorf("--interactive requires -o chart")
	}

	if !term.IsTerminal(int(os.Stdout.Fd())) || !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("--interactive requires a terminal (TTY)")
	}

	if cmd.Flags().Changed("chart-width") || cmd.Flags().Changed("chart-height") {
		return fmt.Errorf("--interactive cannot be combined with --chart-width or --chart-height (uses full terminal)")
	}

	return nil
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

func runShowCategories(cmd *cobra.Command, args []string) error {
	db, dbImpl, err := connectToDB()
	if err != nil {
		return fmt.Errorf("failed to connect to DB: %w", err)
	}
	defer func() { _ = db.Close() }()

	categories, err := dbImpl.ListCategories(db)
	if err != nil {
		return fmt.Errorf("failed to list categories: %w", err)
	}

	if len(categories) == 0 {
		fmt.Println("No categories found.")
		return nil
	}

	records := make([]output.CategoryRecord, len(categories))
	for i, c := range categories {
		records[i] = output.CategoryRecord{
			Category:  c.Category,
			TableName: c.TableName,
			KPICount:  c.KPICount,
		}
	}

	output.PrintCategoriesTable(records)
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
	Category       string
	ClusterName    string
	MetricValue    float64
	TimestampValue float64
	ExecutionTime  time.Time
	MetricLabels   string
}

type KPIQueryParams struct {
	KPIName      string
	Category     string
	ClusterName  string
	LabelFilters map[string]string
	Since        *time.Time
	Until        *time.Time
	Limit        int
	Sort         string
}

func queryKPIs(db *sql.DB, dbImpl database.Database, params KPIQueryParams) ([]KPIResult, error) {
	if params.KPIName != "" {
		return queryByName(db, dbImpl, params)
	}
	if params.Category != "" {
		return queryByCategory(db, dbImpl, params)
	}
	return queryAll(db, dbImpl, params)
}

// queryByName resolves which table holds the KPI via the registry and queries it.
func queryByName(db *sql.DB, dbImpl database.Database, params KPIQueryParams) ([]KPIResult, error) {
	category, tableName, err := dbImpl.LookupCategoryForKPI(db, params.KPIName)
	if err != nil {
		return nil, fmt.Errorf("lookup KPI %q: %w", params.KPIName, err)
	}
	if tableName == "" {
		tableName = database.DefaultTableName
	}
	return queryFromTable(db, dbImpl, tableName, category, params)
}

// queryByCategory queries the single category-specific table.
func queryByCategory(db *sql.DB, dbImpl database.Database, params KPIQueryParams) ([]KPIResult, error) {
	tableName := database.CategoryTableName(params.Category)
	return queryFromTable(db, dbImpl, tableName, params.Category, params)
}

// allCategoryTables returns the default table plus every registered category table.
func allCategoryTables(db *sql.DB, dbImpl database.Database) ([]database.CategoryInfo, error) {
	categories, err := dbImpl.ListCategories(db)
	if err != nil {
		return nil, err
	}
	all := make([]database.CategoryInfo, 0, 1+len(categories))
	all = append(all, database.CategoryInfo{TableName: database.DefaultTableName})
	all = append(all, categories...)
	return all, nil
}

// queryAll scans every known table (default + all categories), merges, sorts, and limits.
// Sort and limit must happen here (not in SQL) because rows come from separate tables —
// we can't know the global ordering or which rows survive the limit until all tables are merged.
func queryAll(db *sql.DB, dbImpl database.Database, params KPIQueryParams) ([]KPIResult, error) {
	tables, err := allCategoryTables(db, dbImpl)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}

	var results []KPIResult
	for _, t := range tables {
		rows, err := queryFromTable(db, dbImpl, t.TableName, t.Category, params)
		if err != nil {
			return nil, err
		}
		results = append(results, rows...)
	}

	sortResults(results, params.Sort)

	if params.Limit > 0 && len(results) > params.Limit {
		results = results[:params.Limit]
	}

	return results, nil
}

func queryFromTable(db *sql.DB, dbImpl database.Database, tableName, category string, params KPIQueryParams) ([]KPIResult, error) {
	query := fmt.Sprintf(`
		SELECT qr.id, qr.kpi_id, c.cluster_name, qr.metric_value,
		       qr.timestamp_value, qr.execution_time, qr.metric_labels
		FROM %s qr
		JOIN clusters c ON qr.cluster_id = c.id
		WHERE 1=1
	`, tableName)

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
	}

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
		r.Category = category

		if len(params.LabelFilters) > 0 && !matchesLabelFilters(r.MetricLabels, params.LabelFilters) {
			continue
		}

		results = append(results, r)
	}

	return results, rows.Err()
}

func sortResults(results []KPIResult, order string) {
	sort.Slice(results, func(i, j int) bool {
		if order == "desc" {
			return results[i].ExecutionTime.After(results[j].ExecutionTime)
		}
		return results[i].ExecutionTime.Before(results[j].ExecutionTime)
	})
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
	tables, err := allCategoryTables(db, dbImpl)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}

	clusterMap := make(map[int64]*ClusterInfo)
	var clusterOrder []int64

	for _, t := range tables {
		query := fmt.Sprintf(`
			SELECT c.id, c.cluster_name, c.created_at, COUNT(qr.id) as total_metrics
			FROM clusters c
			LEFT JOIN %s qr ON c.id = qr.cluster_id
		`, t.TableName)
		args := []interface{}{}

		if clusterName != "" {
			query += " WHERE c.cluster_name = $1"
			args = append(args, clusterName)
		}
		query += " GROUP BY c.id, c.cluster_name, c.created_at"

		if _, ok := dbImpl.(*database.SQLiteDB); ok {
			query = convertPostgresToSQLitePlaceholders(query)
		}

		rows, err := db.Query(query, args...)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var c ClusterInfo
			if err := rows.Scan(&c.ID, &c.Name, &c.CreatedAt, &c.TotalMetrics); err != nil {
				_ = rows.Close()
				return nil, err
			}
			if existing, ok := clusterMap[c.ID]; ok {
				existing.TotalMetrics += c.TotalMetrics
			} else {
				clusterMap[c.ID] = &c
				clusterOrder = append(clusterOrder, c.ID)
			}
		}
		_ = rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	clusters := make([]ClusterInfo, 0, len(clusterOrder))
	for _, id := range clusterOrder {
		clusters = append(clusters, *clusterMap[id])
	}
	return clusters, nil
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

func convertToKPIRecords(results []KPIResult) []output.KPIRecord {
	records := make([]output.KPIRecord, len(results))
	for i, r := range results {
		var labels map[string]string
		_ = json.Unmarshal([]byte(r.MetricLabels), &labels)

		records[i] = output.KPIRecord{
			ID:            r.ID,
			KPIName:       r.KPIName,
			Category:      r.Category,
			Cluster:       r.ClusterName,
			Value:         r.MetricValue,
			Timestamp:     time.Unix(int64(r.TimestampValue), 0).UTC().Format("2006-01-02 15:04:05"),
			ExecutionTime: r.ExecutionTime,
			Labels:        labels,
			LabelsRaw:     r.MetricLabels,
		}
	}
	return records
}

// validateCategory checks that the given category exists in kpi_registry.
// Returns a descriptive error listing available categories when it does not.
func validateCategory(db *sql.DB, dbImpl database.Database, category string) error {
	categories, err := dbImpl.ListCategories(db)
	if err != nil {
		return fmt.Errorf("failed to list categories: %w", err)
	}

	for _, c := range categories {
		if c.Category == category {
			return nil
		}
	}

	if len(categories) == 0 {
		return fmt.Errorf("category %q not found (no categories exist in the database)", category)
	}

	names := make([]string, len(categories))
	for i, c := range categories {
		names[i] = c.Category
	}
	return fmt.Errorf("category %q not found; available categories: %s", category, strings.Join(names, ", "))
}

func convertPostgresToSQLitePlaceholders(query string) string {
	result := query
	for i := 20; i >= 1; i-- {
		result = strings.ReplaceAll(result, fmt.Sprintf("$%d", i), "?")
	}
	return result
}
