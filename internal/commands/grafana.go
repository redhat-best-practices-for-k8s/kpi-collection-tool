package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const grafanaContainerName = "grafana-kpi"

var grafanaCmd = &cobra.Command{
	Use:   "grafana",
	Short: "Manage Grafana dashboard for KPI visualization",
	Long: `Manage a local Grafana instance with the KPI dashboard pre-configured.
Supports both SQLite and PostgreSQL datasources.

Use 'grafana start' to launch Grafana and 'grafana stop' to stop it.`,
}

func init() {
	rootCmd.AddCommand(grafanaCmd)
}

// getGrafanaConfigDir returns the path to the grafana config directory
// ~/.kpi-collector/grafana/
func getGrafanaConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".kpi-collector", "grafana"), nil
}

// createGrafanaDirectories creates all necessary directories for grafana config
func createGrafanaDirectories(grafanaDir string) error {
	dirs := []string{
		grafanaDir,
		filepath.Join(grafanaDir, "datasources"),
		filepath.Join(grafanaDir, "dashboards"),
		filepath.Join(grafanaDir, "provisioning", "dashboards"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
