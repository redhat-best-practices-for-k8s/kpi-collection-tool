package config

import "fmt"

// OslatTaskConfig configures the oslat latency task.
// Exactly one of configFile or inline fields must be set.
type OslatTaskConfig struct {
	ConfigFile       string            `yaml:"configFile,omitempty"`
	Image            string            `yaml:"image,omitempty"`
	ImagePullPolicy  string            `yaml:"imagePullPolicy,omitempty"`
	ImagePullSecret  string            `yaml:"imagePullSecret,omitempty"`
	Namespace        string            `yaml:"namespace,omitempty"`
	PodName          string            `yaml:"podName,omitempty"`
	Runtime          Duration          `yaml:"runtime,omitempty"`
	InitialDelay     Duration          `yaml:"initialDelay,omitempty"`
	Delay            Duration          `yaml:"delay,omitempty"`
	RuntimeClassName string            `yaml:"runtimeClassName,omitempty"`
	NodeSelector     map[string]string `yaml:"nodeSelector,omitempty"`
	CPU              string            `yaml:"cpu,omitempty"`
	Memory           string            `yaml:"memory,omitempty"`
	Replicas         int               `yaml:"replicas,omitempty"`
	Timeout          Duration          `yaml:"timeout,omitempty"`
}

func (c *OslatTaskConfig) hasInlineFields() bool {
	return c.Image != "" ||
		c.ImagePullPolicy != "" ||
		c.ImagePullSecret != "" ||
		c.Namespace != "" ||
		c.PodName != "" ||
		c.Runtime.Duration != 0 ||
		c.InitialDelay.Duration != 0 ||
		c.Delay.Duration != 0 ||
		c.RuntimeClassName != "" ||
		len(c.NodeSelector) > 0 ||
		c.CPU != "" ||
		c.Memory != "" ||
		c.Replicas != 0 ||
		c.Timeout.Duration != 0
}

func resolveAndValidateOslat(spec *TasksSpec) error {
	if spec.Oslat == nil {
		return nil
	}
	cfg := spec.Oslat
	if err := validateConfigFileXorInline(TaskConfigOslat, cfg.ConfigFile != "", cfg.hasInlineFields()); err != nil {
		return err
	}
	if cfg.ConfigFile != "" {
		loaded, err := loadOslatConfigFile(spec.ResolvePath(cfg.ConfigFile))
		if err != nil {
			return fmt.Errorf("%s: %w", TaskConfigOslat, err)
		}
		spec.Oslat = loaded
	}
	return validateOslatInline(spec.Oslat)
}

func loadOslatConfigFile(path string) (*OslatTaskConfig, error) {
	var wrap struct {
		Oslat *OslatTaskConfig `yaml:"oslat"`
	}
	if err := unmarshalWrappedConfig(path, &wrap); err != nil {
		return nil, err
	}
	if wrap.Oslat == nil {
		return nil, missingRootKeyError(path, TaskConfigOslat)
	}
	if wrap.Oslat.ConfigFile != "" {
		return nil, nestedConfigFileError(TaskConfigOslat)
	}
	return wrap.Oslat, nil
}

func validateOslatInline(cfg *OslatTaskConfig) error {
	if cfg.Image == "" {
		return fmt.Errorf("%s: image is required", TaskConfigOslat)
	}
	if cfg.Runtime.Duration <= 0 {
		return fmt.Errorf("%s: runtime must be greater than 0", TaskConfigOslat)
	}
	if cfg.CPU == "" {
		return fmt.Errorf("%s: cpu is required", TaskConfigOslat)
	}
	if cfg.Memory == "" {
		return fmt.Errorf("%s: memory is required", TaskConfigOslat)
	}
	if cfg.Replicas < 1 {
		return fmt.Errorf("%s: replicas must be at least 1", TaskConfigOslat)
	}
	if cfg.Timeout.Duration <= 0 {
		return fmt.Errorf("%s: timeout must be greater than 0", TaskConfigOslat)
	}
	if cfg.Timeout.Duration <= cfg.Runtime.Duration {
		return fmt.Errorf("%s: timeout must be greater than runtime", TaskConfigOslat)
	}
	return nil
}
