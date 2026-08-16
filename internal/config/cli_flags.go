package config

import (
	"fmt"
)

// ValidateFlags ensures the correct combination of flags is provided
func ValidateFlags(flags InputFlags) error {
	if flags.ClusterName == "" {
		return fmt.Errorf("cluster name is required: use --cluster-name flag")
	}

	validClusterTypes := map[string]bool{"ran": true, "core": true, "hub": true}
	if flags.ClusterType == "" {
		return fmt.Errorf("cluster-type is required: must be 'ran', 'core', or 'hub'")
	}
	if !validClusterTypes[flags.ClusterType] {
		return fmt.Errorf("invalid cluster-type '%s': must be 'ran', 'core', or 'hub'", flags.ClusterType)
	}

	if flags.InsecureTLS {
		fmt.Println("WARNING: TLS certificate verification is disabled. Use only in development environments.")
	}

	validAuthCombo := (flags.BearerToken != "" && flags.ThanosURL != "" && flags.Kubeconfig == "") ||
		(flags.BearerToken == "" && flags.ThanosURL == "" && flags.Kubeconfig != "")

	if !validAuthCombo {
		return fmt.Errorf("invalid flag combination: either provide --token and --thanos-url, or provide --kubeconfig")
	}

	if !flags.HasAnyTaskConfig() && flags.BundleConfig == "" {
		return fmt.Errorf("at least one task config flag or --bundle is required")
	}

	if flags.SamplingFreq <= 0 {
		return fmt.Errorf("sampling frequency must be greater than 0")
	}

	if flags.Duration <= 0 {
		return fmt.Errorf("duration must be greater than 0")
	}

	if flags.DatabaseType != "sqlite" && flags.DatabaseType != "postgres" {
		return fmt.Errorf("invalid db-type: must be 'sqlite' or 'postgres'")
	}

	if flags.DatabaseType == "postgres" && flags.PostgresURL == "" {
		return fmt.Errorf("postgres-url is required when db-type=postgres")
	}

	return nil
}
