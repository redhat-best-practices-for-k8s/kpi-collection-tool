package commands

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

var grafanaStopCmd = &cobra.Command{
	Use:     "stop",
	Short:   "Stop Grafana dashboard",
	Long:    `Stop the running Grafana container (grafana-kpi).`,
	Example: `  kpi-collector grafana stop`,
	RunE:    runGrafanaStop,
}

func init() {
	grafanaCmd.AddCommand(grafanaStopCmd)
}

func runGrafanaStop(cmd *cobra.Command, args []string) error {
	fmt.Printf("Stopping Grafana container (%s)...\n", grafanaContainerName)

	// Stop the container
	stopCmd := exec.Command("docker", "stop", grafanaContainerName)
	if output, err := stopCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stop Grafana: %w\nOutput: %s", err, string(output))
	}

	// Remove the container
	rmCmd := exec.Command("docker", "rm", grafanaContainerName)
	if output, err := rmCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove Grafana container: %w\nOutput: %s", err, string(output))
	}

	fmt.Println("✅ Grafana stopped and removed.")
	return nil
}

