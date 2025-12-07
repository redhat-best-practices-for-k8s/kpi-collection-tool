package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	grafana_templates "kpi-collector/grafana-templates"
	"kpi-collector/internal/database"

	"github.com/spf13/cobra"
)

var grafanaStartFlags struct {
	datasource  string
	postgresURL string
	port        int
}

var grafanaStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Grafana dashboard",
	Long: `Start a local Grafana instance with the KPI dashboard pre-configured.
Generates configuration files in ~/.kpi-collector/grafana/ and
launches Grafana via Docker with all necessary volume mounts.`,
	Example: `  # Using SQLite
  kpi-collector grafana start --datasource=sqlite

  # Using PostgreSQL
  kpi-collector grafana start --datasource=postgres --postgres-url "postgresql://user:pass@host:5432/dbname"

  # Custom port
  kpi-collector grafana start --datasource=sqlite --port 3001`,
	RunE: runGrafanaStart,
}

func init() {
	grafanaCmd.AddCommand(grafanaStartCmd)

	grafanaStartCmd.Flags().StringVar(&grafanaStartFlags.datasource, "datasource", "",
		"datasource type: sqlite or postgres (required)")
	grafanaStartCmd.Flags().StringVar(&grafanaStartFlags.postgresURL, "postgres-url", "",
		"PostgreSQL connection string (required if datasource=postgres)")
	grafanaStartCmd.Flags().IntVar(&grafanaStartFlags.port, "port", 3000,
		"Grafana port (default: 3000)")

	if err := grafanaStartCmd.MarkFlagRequired("datasource"); err != nil {
		panic(fmt.Sprintf("failed to mark datasource as required: %v", err))
	}
}

func runGrafanaStart(cmd *cobra.Command, args []string) error {
	if grafanaStartFlags.datasource != "sqlite" && grafanaStartFlags.datasource != "postgres" {
		return fmt.Errorf("datasource must be 'sqlite' or 'postgres', got: %s", grafanaStartFlags.datasource)
	}

	if grafanaStartFlags.datasource == "postgres" && grafanaStartFlags.postgresURL == "" {
		return fmt.Errorf("--postgres-url is required when datasource is 'postgres'")
	}

	fmt.Printf("Starting Grafana with %s datasource...\n", grafanaStartFlags.datasource)

	// Get the grafana config directory
	grafanaDir, err := getGrafanaConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get grafana config directory: %w", err)
	}

	// Create all necessary directories
	if err := createGrafanaDirectories(grafanaDir); err != nil {
		return fmt.Errorf("failed to create grafana directories: %w", err)
	}

	// Write all configuration files from embedded templates
	if err := writeGrafanaConfigFiles(grafanaDir); err != nil {
		return fmt.Errorf("failed to write grafana config files: %w", err)
	}

	// Run Docker directly
	if err := runGrafanaDocker(grafanaDir); err != nil {
		return fmt.Errorf("failed to start Grafana: %w", err)
	}

	fmt.Println("✅ Grafana is starting up...")
	fmt.Printf("🌐 Open http://localhost:%d in your browser\n", grafanaStartFlags.port)
	fmt.Println("👤 Default login: admin/admin")
	fmt.Printf("\n💡 To stop Grafana, run: kpi-collector grafana stop\n")

	return nil
}

// writeGrafanaConfigFiles writes all configuration files from embedded templates
func writeGrafanaConfigFiles(grafanaDir string) error {
	// Write dashboard JSON
	dashboardFile := grafana_templates.SQLiteDashboard
	if grafanaStartFlags.datasource == "postgres" {
		dashboardFile = grafana_templates.PostgresDashboard
	}

	dashboardContent, err := grafana_templates.FS.ReadFile(dashboardFile)
	if err != nil {
		return fmt.Errorf("failed to read embedded dashboard template: %w", err)
	}

	dashboardPath := filepath.Join(grafanaDir, "dashboards", "dashboard.json")
	if err := os.WriteFile(dashboardPath, dashboardContent, 0644); err != nil {
		return fmt.Errorf("failed to write dashboard file: %w", err)
	}

	// Write provisioning config for dashboards
	provisioningContent, err := grafana_templates.FS.ReadFile(grafana_templates.ProvisioningDashboards)
	if err != nil {
		return fmt.Errorf("failed to read embedded provisioning template: %w", err)
	}

	provisioningPath := filepath.Join(grafanaDir, "provisioning", "dashboards", "dashboards.yaml")
	if err := os.WriteFile(provisioningPath, provisioningContent, 0644); err != nil {
		return fmt.Errorf("failed to write provisioning file: %w", err)
	}

	// Write datasource config
	datasourceContent := generateDatasourceConfig()
	datasourcePath := filepath.Join(grafanaDir, "datasources", "datasource.yaml")
	if err := os.WriteFile(datasourcePath, []byte(datasourceContent), 0644); err != nil {
		return fmt.Errorf("failed to write datasource file: %w", err)
	}

	return nil
}

// generateDatasourceConfig generates the datasource YAML based on the datasource type
func generateDatasourceConfig() string {
	if grafanaStartFlags.datasource == "sqlite" {
		return `apiVersion: 1

datasources:
  - name: KPI-SQLite
    type: frser-sqlite-datasource
    uid: kpi-datasource
    access: proxy
    jsonData:
      path: /var/lib/grafana/kpi_metrics.db
    isDefault: true
    editable: true
`
	}

	// PostgreSQL datasource
	pgURL := grafanaStartFlags.postgresURL
	pgURL = strings.TrimPrefix(pgURL, "postgresql://")
	pgURL = strings.TrimPrefix(pgURL, "postgres://")

	var user, password, host, port, dbName string

	// Extract user and password
	atIndex := strings.Index(pgURL, "@")
	if atIndex > 0 {
		userPass := pgURL[:atIndex]
		pgURL = pgURL[atIndex+1:]

		colonIndex := strings.Index(userPass, ":")
		if colonIndex > 0 {
			user = userPass[:colonIndex]
			password = userPass[colonIndex+1:]
		} else {
			user = userPass
		}
	}

	// Extract database
	slashIndex := strings.Index(pgURL, "/")
	if slashIndex > 0 {
		dbName = pgURL[slashIndex+1:]
		if qIndex := strings.Index(dbName, "?"); qIndex > 0 {
			dbName = dbName[:qIndex]
		}
		pgURL = pgURL[:slashIndex]
	}

	// Extract host and port
	colonIndex := strings.LastIndex(pgURL, ":")
	if colonIndex > 0 {
		host = pgURL[:colonIndex]
		port = pgURL[colonIndex+1:]
	} else {
		host = pgURL
		port = "5432"
	}

	config := fmt.Sprintf(`apiVersion: 1

datasources:
  - name: KPI-PostgreSQL
    type: postgres
    uid: kpi-datasource
    access: proxy
    url: %s:%s
    database: %s
    user: %s
    isDefault: true
    editable: true
    jsonData:
      sslmode: disable
      maxOpenConns: 10
      maxIdleConns: 10
      connMaxLifetime: 14400
`, host, port, dbName, user)

	if password != "" {
		config += fmt.Sprintf(`    secureJsonData:
      password: %s
`, password)
	}

	return config
}

// runGrafanaDocker runs Grafana via Docker with all necessary volume mounts
func runGrafanaDocker(grafanaDir string) error {
	// Stop and remove existing container if it exists
	stopCmd := exec.Command("docker", "rm", "-f", grafanaContainerName)
	_ = stopCmd.Run() // Ignore error if container doesn't exist

	// Build docker run command
	args := []string{
		"run", "-d",
		"--name", grafanaContainerName,
		"-p", fmt.Sprintf("%d:3000", grafanaStartFlags.port),
		// Mount datasource config
		"-v", fmt.Sprintf("%s:/etc/grafana/provisioning/datasources:ro",
			filepath.Join(grafanaDir, "datasources")),
		// Mount dashboard provisioning config
		"-v", fmt.Sprintf("%s:/etc/grafana/provisioning/dashboards:ro",
			filepath.Join(grafanaDir, "provisioning", "dashboards")),
		// Mount dashboard JSON
		"-v", fmt.Sprintf("%s:/var/lib/grafana/dashboards:ro",
			filepath.Join(grafanaDir, "dashboards")),
	}

	// For SQLite, mount the database file
	if grafanaStartFlags.datasource == "sqlite" {
		dbPath := database.GetSQLiteDBPath()
		args = append(args,
			"-v", fmt.Sprintf("%s:/var/lib/grafana/kpi_metrics.db:ro", dbPath),
			"-e", "GF_INSTALL_PLUGINS=frser-sqlite-datasource",
		)
	}

	// Add the image name
	args = append(args, "grafana/grafana:latest")

	dockerCmd := exec.Command("docker", args...)
	output, err := dockerCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker run failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}
