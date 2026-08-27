package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func validateConfigFileXorInline(taskName string, hasConfigFile, hasInline bool) error {
	if hasConfigFile && hasInline {
		return fmt.Errorf("%s: configFile and inline fields are mutually exclusive — set only one", taskName)
	}
	if !hasConfigFile && !hasInline {
		return fmt.Errorf("%s: one of configFile or inline fields must be set", taskName)
	}
	return nil
}

func unmarshalWrappedConfig(path string, dest any) error {
	data, err := os.ReadFile(path) //#nosec G304 -- path is user-provided CLI input
	if err != nil {
		return fmt.Errorf("failed to open config file %q: %w", path, err)
	}
	if err := yaml.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("failed to decode config file %q: %w", path, err)
	}
	return nil
}

func missingRootKeyError(path, rootKey string) error {
	return fmt.Errorf("config file %q must have an %q root key", path, rootKey)
}

func nestedConfigFileError(taskName string) error {
	return fmt.Errorf("configFile is not allowed inside a %s config file", taskName)
}

func validatePollConfig(taskName string, namespaces []string, interval, duration Duration) error {
	if len(namespaces) == 0 {
		return fmt.Errorf("%s: workloadNamespaces is required", taskName)
	}
	if duration.Duration <= 0 {
		return fmt.Errorf("%s: duration must be greater than 0", taskName)
	}
	if interval.Duration <= 0 {
		return fmt.Errorf("%s: interval must be greater than 0", taskName)
	}
	if interval.Duration >= duration.Duration {
		return fmt.Errorf("%s: interval must be less than duration", taskName)
	}
	return nil
}
