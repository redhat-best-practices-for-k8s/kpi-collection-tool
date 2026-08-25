package config

import "fmt"

const (
	NodesAll    = "all"
	NodesMaster = "master"
	NodesWorker = "worker"
)

// PerNodeDataTaskConfig configures the per-node data collection task.
// Exactly one of ConfigFile or inline fields must be set.
type PerNodeDataTaskConfig struct {
	ConfigFile         string   `yaml:"configFile,omitempty"`
	Duration           Duration `yaml:"duration,omitempty"`
	Interval           Duration `yaml:"interval,omitempty"`
	Nodes              string   `yaml:"nodes,omitempty"`
	NodeNames          []string `yaml:"nodeNames,omitempty"`
	SSHUser            string   `yaml:"sshUser,omitempty"`
	SSHPort            int      `yaml:"sshPort,omitempty"`
	SSHKey             string   `yaml:"sshKey,omitempty"`
	RemoteWorkDir      string   `yaml:"remoteWorkDir,omitempty"`
	Isolcpus           string   `yaml:"isolcpus,omitempty"`
	WorkloadNamespaces []string `yaml:"workloadNamespaces,omitempty"`
	CollectTop         *bool    `yaml:"collectTop,omitempty"`
	CollectProc        *bool    `yaml:"collectProc,omitempty"`
	CollectDescribe    *bool    `yaml:"collectDescribe,omitempty"`
}

func resolveAndValidatePerNodeData(spec *TasksSpec) error {
	if spec.PerNodeData == nil {
		return nil
	}
	resolved, err := resolveTaskConfig(spec, TaskConfigPerNodeData, spec.PerNodeData.ConfigFile, spec.PerNodeData, loadPerNodeDataConfigFile)
	if err != nil {
		return err
	}
	spec.PerNodeData = resolved
	return validatePerNodeDataInline(spec.PerNodeData)
}

func loadPerNodeDataConfigFile(path string) (*PerNodeDataTaskConfig, error) {
	var wrap struct {
		PerNodeData *PerNodeDataTaskConfig `yaml:"per-node-data"`
	}
	if err := unmarshalWrappedConfig(path, &wrap); err != nil {
		return nil, err
	}
	if wrap.PerNodeData == nil {
		return nil, missingRootKeyError(path, TaskConfigPerNodeData)
	}
	if wrap.PerNodeData.ConfigFile != "" {
		return nil, nestedConfigFileError(TaskConfigPerNodeData)
	}
	return wrap.PerNodeData, nil
}

func validatePerNodeDataInline(cfg *PerNodeDataTaskConfig) error {
	if err := validatePollConfig(TaskConfigPerNodeData, cfg.WorkloadNamespaces, cfg.Interval, cfg.Duration); err != nil {
		return err
	}
	if cfg.Nodes == "" {
		return fmt.Errorf("%s: nodes is required", TaskConfigPerNodeData)
	}
	switch cfg.Nodes {
	case NodesAll, NodesMaster, NodesWorker:
	default:
		return fmt.Errorf("%s: nodes %q is invalid: must be %q, %q, or %q",
			TaskConfigPerNodeData, cfg.Nodes, NodesAll, NodesMaster, NodesWorker)
	}
	if cfg.SSHUser == "" {
		return fmt.Errorf("%s: sshUser is required", TaskConfigPerNodeData)
	}
	if cfg.SSHPort == 0 {
		return fmt.Errorf("%s: sshPort is required", TaskConfigPerNodeData)
	}
	if cfg.RemoteWorkDir == "" {
		return fmt.Errorf("%s: remoteWorkDir is required", TaskConfigPerNodeData)
	}
	return nil
}
