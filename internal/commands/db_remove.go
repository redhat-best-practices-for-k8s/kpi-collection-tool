package commands

import (
	"database/sql"
	"fmt"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/database"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

var (
	removeClusterName string
	removeKPIName     string
	removeCategory    string
	removeAll         bool
)

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Delete data from the database",
	Long: `Delete clusters, KPI metrics, or error records from the database.

WARNING: All remove operations are immediate and cannot be undone.`,
}

var removeClustersCmd = &cobra.Command{
	Use:   "clusters",
	Short: "Remove a cluster and all its data",
	Long: `Delete a cluster record and all associated KPI metrics from the database.

WARNING: This operation cannot be undone. All metric samples for the cluster will be permanently deleted.`,
	Example: `  # Remove a cluster
  kpi-collector db remove clusters --name="old-cluster"`,
	RunE: runRemoveClusters,
}

var removeKPIsCmd = &cobra.Command{
	Use:   "kpis",
	Short: "Remove KPI metrics",
	Long: `Delete KPI metrics from the database, filtered by cluster, KPI name, or category.

When --category is used, all data in that category table is removed along with
the registry entries. This is independent of the --cluster-name flag.`,
	Example: `  # Remove all KPIs from a cluster
  kpi-collector db remove kpis --cluster-name="mycluster1"
  
  # Remove specific KPI from a cluster
  kpi-collector db remove kpis --cluster-name="mycluster1" --name="cpu-system"

  # Remove all data for a category
  kpi-collector db remove kpis --category=cpu`,
	RunE: runRemoveKPIs,
}

var removeErrorsCmd = &cobra.Command{
	Use:   "errors",
	Short: "Clear error counts",
	Long:  `Reset error counts for KPI queries.`,
	Example: `  # Clear errors for a specific KPI
  kpi-collector db remove errors --name="cpu-system"
  
  # Clear all errors
  kpi-collector db remove errors --all`,
	RunE: runRemoveErrors,
}

func init() {
	dbCmd.AddCommand(removeCmd)
	removeCmd.AddCommand(removeClustersCmd)
	removeCmd.AddCommand(removeKPIsCmd)
	removeCmd.AddCommand(removeErrorsCmd)

	// Flags for 'remove clusters'
	removeClustersCmd.Flags().StringVar(&removeClusterName, "name", "",
		"cluster name to remove (required)")
	_ = removeClustersCmd.MarkFlagRequired("name")

	// Flags for 'remove kpis'
	removeKPIsCmd.Flags().StringVar(&removeClusterName, "cluster-name", "",
		"cluster name (required unless --category is used)")
	removeKPIsCmd.Flags().StringVar(&removeKPIName, "name", "",
		"KPI name to remove (optional)")
	removeKPIsCmd.Flags().StringVar(&removeCategory, "category", "",
		"remove all data for a category (e.g. cpu, memory)")

	// Flags for 'remove errors'
	removeErrorsCmd.Flags().StringVar(&removeKPIName, "name", "",
		"KPI name to clear errors for")
	removeErrorsCmd.Flags().BoolVar(&removeAll, "all", false,
		"clear all error records")
}

func runRemoveClusters(cmd *cobra.Command, args []string) error {
	db, dbImpl, err := connectToDB()
	if err != nil {
		return fmt.Errorf("failed to connect to DB: %w", err)
	}
	defer func() { _ = db.Close() }()

	cluster, metricsDeleted, err := deleteClusterMetrics(db, dbImpl, removeClusterName, "")
	if err != nil {
		return err
	}

	deleteQuery := "DELETE FROM clusters WHERE id = $1"
	if _, ok := dbImpl.(*database.SQLiteDB); ok {
		deleteQuery = convertPostgresToSQLitePlaceholders(deleteQuery)
	}

	_, err = db.Exec(deleteQuery, cluster.ID)
	if err != nil {
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	fmt.Printf("✓ Deleted cluster '%s' and %s metric samples.\n",
		cluster.Name, humanize.Comma(metricsDeleted))
	return nil
}

func runRemoveKPIs(cmd *cobra.Command, args []string) error {
	// user must provide the cluster namr OR a category to remove KPIs
	if removeCategory == "" && removeClusterName == "" {
		return fmt.Errorf("must specify either --cluster-name or --category")
	}

	db, dbImpl, err := connectToDB()
	if err != nil {
		return fmt.Errorf("failed to connect to DB: %w", err)
	}
	defer func() { _ = db.Close() }()

	if removeCategory != "" {
		deleted, err := dbImpl.DeleteByCategory(db, removeCategory)
		if err != nil {
			return fmt.Errorf("failed to delete category '%s': %w", removeCategory, err)
		}
		if deleted == 0 {
			fmt.Printf("No metrics found for category '%s'.\n", removeCategory)
			return nil
		}
		fmt.Printf("✓ Deleted %s metric samples from category '%s'.\n", humanize.Comma(deleted), removeCategory)
		return nil
	}

	_, deleted, err := deleteClusterMetrics(db, dbImpl, removeClusterName, removeKPIName)
	if err != nil {
		return err
	}

	if deleted == 0 {
		fmt.Println("No metrics found matching the criteria.")
		return nil
	}

	fmt.Printf("✓ Deleted %s metric samples.\n", humanize.Comma(deleted))
	return nil
}

func runRemoveErrors(cmd *cobra.Command, args []string) error {
	db, dbImpl, err := connectToDB()
	if err != nil {
		return fmt.Errorf("failed to connect to DB: %w", err)
	}
	defer func() { _ = db.Close() }()

	var query string
	var queryArgs []interface{}

	if removeAll {
		query = "DELETE FROM query_errors"
	} else if removeKPIName != "" {
		query = "DELETE FROM query_errors WHERE kpi_id = $1"
		queryArgs = append(queryArgs, removeKPIName)
	} else {
		return fmt.Errorf("must specify either --name or --all")
	}

	if _, ok := dbImpl.(*database.SQLiteDB); ok {
		query = convertPostgresToSQLitePlaceholders(query)
	}

	result, err := db.Exec(query, queryArgs...)
	if err != nil {
		return fmt.Errorf("failed to delete errors: %w", err)
	}

	deleted, _ := result.RowsAffected()
	if deleted == 0 {
		fmt.Println("No error records found.")
		return nil
	}

	fmt.Printf("✓ Cleared %d error record(s).\n", deleted)
	return nil
}

func deleteClusterMetrics(db *sql.DB, dbImpl database.Database, clusterName, kpiName string) (*ClusterInfo, int64, error) {
	clusters, err := listClusters(db, dbImpl, clusterName)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query cluster: %w", err)
	}
	if len(clusters) == 0 {
		return nil, 0, fmt.Errorf("cluster '%s' not found", clusterName)
	}
	cluster := clusters[0]

	tables, err := allCategoryTables(db, dbImpl)
	if err != nil {
		return nil, 0, fmt.Errorf("list tables: %w", err)
	}

	var totalDeleted int64
	for _, t := range tables {
		query := fmt.Sprintf("DELETE FROM %s WHERE cluster_id = $1", t.TableName)
		queryArgs := []interface{}{cluster.ID}

		if kpiName != "" {
			query += " AND kpi_id = $2"
			queryArgs = append(queryArgs, kpiName)
		}

		if _, ok := dbImpl.(*database.SQLiteDB); ok {
			query = convertPostgresToSQLitePlaceholders(query)
		}

		result, err := db.Exec(query, queryArgs...)
		if err != nil {
			return nil, totalDeleted, fmt.Errorf("failed to delete from %s: %w", t.TableName, err)
		}

		deleted, _ := result.RowsAffected()
		totalDeleted += deleted
	}

	return &cluster, totalDeleted, nil
}
