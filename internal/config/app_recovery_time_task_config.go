package config

import "fmt"

const (
	StartWhenNodeUnreachable = "node-unreachable"
	StartWhenImmediate       = "immediate"
)

// AppRecoveryTimeTaskConfig configures the application recovery-time task.
// Exactly one of configFile or inline fields must be set.
type AppRecoveryTimeTaskConfig struct {
	ConfigFile         string   `yaml:"configFile,omitempty"`
	WorkloadNamespaces []string `yaml:"workloadNamespaces,omitempty"`
	Duration           Duration `yaml:"duration,omitempty"`
	Interval           Duration `yaml:"interval,omitempty"`
	StartWhen          string   `yaml:"startWhen,omitempty"`
	NodeNames          []string `yaml:"nodeNames,omitempty"`
	Reboot             *bool    `yaml:"reboot,omitempty"`
}

func (c *AppRecoveryTimeTaskConfig) hasInlineFields() bool {
	return len(c.WorkloadNamespaces) > 0 ||
		c.Duration.Duration != 0 ||
		c.Interval.Duration != 0 ||
		c.StartWhen != "" ||
		len(c.NodeNames) > 0 ||
		c.Reboot != nil
}

func resolveAndValidateAppRecoveryTime(spec *TasksSpec) error {
	if spec.AppRecoveryTime == nil {
		return nil
	}
	cfg := spec.AppRecoveryTime
	if err := validateConfigFileXorInline(TaskConfigAppRecoveryTime, cfg.ConfigFile != "", cfg.hasInlineFields()); err != nil {
		return err
	}
	if cfg.ConfigFile != "" {
		loaded, err := loadAppRecoveryTimeConfigFile(spec.ResolvePath(cfg.ConfigFile))
		if err != nil {
			return fmt.Errorf("%s: %w", TaskConfigAppRecoveryTime, err)
		}
		spec.AppRecoveryTime = loaded
	}
	return validateAppRecoveryTimeInline(spec.AppRecoveryTime)
}

func loadAppRecoveryTimeConfigFile(path string) (*AppRecoveryTimeTaskConfig, error) {
	var wrap struct {
		AppRecoveryTime *AppRecoveryTimeTaskConfig `yaml:"app-recovery-time"`
	}
	if err := unmarshalWrappedConfig(path, &wrap); err != nil {
		return nil, err
	}
	if wrap.AppRecoveryTime == nil {
		return nil, missingRootKeyError(path, TaskConfigAppRecoveryTime)
	}
	if wrap.AppRecoveryTime.ConfigFile != "" {
		return nil, nestedConfigFileError(TaskConfigAppRecoveryTime)
	}
	return wrap.AppRecoveryTime, nil
}

func validateAppRecoveryTimeInline(cfg *AppRecoveryTimeTaskConfig) error {
	if err := validatePollConfig(TaskConfigAppRecoveryTime, cfg.WorkloadNamespaces, cfg.Interval, cfg.Duration); err != nil {
		return err
	}
	if cfg.StartWhen == "" {
		return fmt.Errorf("%s: startWhen is required", TaskConfigAppRecoveryTime)
	}
	switch cfg.StartWhen {
	case StartWhenNodeUnreachable, StartWhenImmediate:
		return nil
	default:
		return fmt.Errorf("%s: startWhen %q is invalid: must be %q or %q",
			TaskConfigAppRecoveryTime, cfg.StartWhen, StartWhenNodeUnreachable, StartWhenImmediate)
	}
}
