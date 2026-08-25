package config

import (
	"fmt"
	"os"
	"reflect"

	"gopkg.in/yaml.v3"
)

func resolveTaskConfig[T any](
	spec *TasksSpec,
	taskName, configFile string,
	section *T,
	load func(string) (*T, error),
) (*T, error) {
	hasFile := configFile != ""
	hasInline := hasNonConfigFileFields(section)
	if hasFile && hasInline {
		return nil, fmt.Errorf("%s: configFile and inline fields are mutually exclusive — set only one", taskName)
	}
	if !hasFile && !hasInline {
		return nil, fmt.Errorf("%s: one of configFile or inline fields must be set", taskName)
	}
	if !hasFile {
		return section, nil
	}
	loaded, err := load(spec.ResolvePath(configFile))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", taskName, err)
	}
	return loaded, nil
}

func hasNonConfigFileFields(section any) bool {
	value := reflect.ValueOf(section)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}
	structType := value.Type()
	for i := 0; i < value.NumField(); i++ {
		if structType.Field(i).Name == "ConfigFile" {
			continue
		}
		if !value.Field(i).IsZero() {
			return true
		}
	}
	return false
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
