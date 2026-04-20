package commands

import "github.com/spf13/cobra"

var kpisCmd = &cobra.Command{
	Use:   "kpis",
	Short: "Manage KPI configuration files",
	Long: `Create and manage KPI configuration files for different cluster profiles.

Use 'kpis generate' to generate a kpis.json file tailored for your cluster type.`,
}

func init() {
	rootCmd.AddCommand(kpisCmd)
}
