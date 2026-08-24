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
	if spec.Prometheus == nil {
		return nil, fmt.Errorf("%s: task config is missing", config.TaskConfigPrometheus)
	}

	kpis := config.KPIs{Queries: spec.Prometheus.Kpis}
	if spec.Prometheus.ConfigFile != "" {
		var err error
		kpis, err = config.LoadKPIs(spec.ResolvePath(spec.Prometheus.ConfigFile))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", config.TaskConfigPrometheus, err)
		}
	}

	return NewPromKPITask(kpis, flags), nil
}

// Replace the stub bodies when each task's YAML schema is designed.

func buildOslatTask(spec config.TasksSpec, flags config.InputFlags) (Task, error) {
	return notYetSupported(config.TaskConfigOslat, spec, flags)
}

func buildPerNodeDataTask(spec config.TasksSpec, flags config.InputFlags) (Task, error) {
	return notYetSupported(config.TaskConfigPerNodeData, spec, flags)
}

func buildAppRecoveryTimeTask(spec config.TasksSpec, flags config.InputFlags) (Task, error) {
	return notYetSupported(config.TaskConfigAppRecoveryTime, spec, flags)
}

func notYetSupported(name string, _ config.TasksSpec, _ config.InputFlags) (Task, error) {
	return nil, fmt.Errorf("%s: task type not yet supported", name)
}
