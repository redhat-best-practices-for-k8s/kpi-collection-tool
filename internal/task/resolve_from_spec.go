package task

import (
	"fmt"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
)

type taskBuilder func(config.TasksSpec, config.InputFlags) (Task, error)

var taskBuilders = map[string]taskBuilder{
	config.TaskConfigPrometheus:      buildPrometheusTask,
	config.TaskConfigOslat:           buildOslatTask,
	config.TaskConfigPerNodeData:     buildPerNodeDataTask,
	config.TaskConfigAppRecoveryTime: buildAppRecoveryTimeTask,
}

// ResolveFromTasksSpec builds the ordered list of tasks described by a TasksSpec.
// Unsupported task types return a "not yet supported" error instead of a Task.
func ResolveFromTasksSpec(spec config.TasksSpec, flags config.InputFlags) ([]Task, error) {
	if err := config.ValidateTasksSpec(&spec); err != nil {
		return nil, err
	}

	var tasks []Task
	for _, taskName := range spec.Orchestration.Order {
		build, ok := taskBuilders[taskName]
		if !ok {
			return nil, fmt.Errorf("%s: unknown task config", taskName)
		}
		t, err := build(spec, flags)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func buildPrometheusTask(spec config.TasksSpec, flags config.InputFlags) (Task, error) {
	kpis, err := LoadPrometheusKPIs(spec)
	if err != nil {
		return nil, err
	}
	return NewPromKPITask(kpis, flags), nil
}

// LoadPrometheusKPIs reads prometheus queries from inline kpis or configFile.
func LoadPrometheusKPIs(spec config.TasksSpec) (config.KPIs, error) {
	if spec.Prometheus == nil {
		return config.KPIs{}, fmt.Errorf("%s: task config is missing", config.TaskConfigPrometheus)
	}

	if spec.Prometheus.ConfigFile != "" {
		kpis, err := config.LoadKPIs(spec.ResolvePath(spec.Prometheus.ConfigFile))
		if err != nil {
			return config.KPIs{}, fmt.Errorf("%s: %w", config.TaskConfigPrometheus, err)
		}
		return kpis, nil
	}

	return config.KPIs{Queries: spec.Prometheus.Kpis}, nil
}

// FromPromKPIsFlag builds the single prometheus task used by --prom-kpis-config.
func FromPromKPIsFlag(flags config.InputFlags, kpis config.KPIs) ([]Task, error) {
	if flags.PromKPIsConfig == "" {
		return nil, fmt.Errorf("no tasks configured: --prom-kpis-config is required")
	}
	return []Task{NewPromKPITask(kpis, flags)}, nil
}

func buildOslatTask(_ config.TasksSpec, _ config.InputFlags) (Task, error) {
	return nil, errNotYetSupported(config.TaskConfigOslat)
}

func buildPerNodeDataTask(_ config.TasksSpec, _ config.InputFlags) (Task, error) {
	return nil, errNotYetSupported(config.TaskConfigPerNodeData)
}

func buildAppRecoveryTimeTask(_ config.TasksSpec, _ config.InputFlags) (Task, error) {
	return nil, errNotYetSupported(config.TaskConfigAppRecoveryTime)
}
