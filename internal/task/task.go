// Package task defines runnable units executed by `kpi-collector run`.
// Today only the Prometheus KPI task is supported; more task types can be
// added later without rewriting the run command.
package task

import (
	"context"
	"fmt"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/collector"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
)

// Task is a single unit of work that `run` can execute.
type Task interface {
	Name() string
	Run(ctx context.Context) error
}

// PromKPITask collects Prometheus/Thanos KPI metrics.
type PromKPITask struct {
	kpis  config.KPIs
	flags config.InputFlags
}

// NewPromKPITask creates a Prom KPI collection task.
func NewPromKPITask(kpis config.KPIs, flags config.InputFlags) *PromKPITask {
	return &PromKPITask{kpis: kpis, flags: flags}
}

// Name returns the task identifier.
func (t *PromKPITask) Name() string {
	return "prom-kpis"
}

// Run executes Prom KPI collection (once or periodic based on flags).
func (t *PromKPITask) Run(ctx context.Context) error {
	_ = ctx // reserved for cancellation in later task types

	if t.flags.SingleRun {
		return collector.RunKPIsOnce(t.kpis, t.flags)
	}
	return collector.RunKPIs(t.kpis, t.flags)
}

// ResolveFromFlags builds the task list from whichever config flags are set.
func ResolveFromFlags(flags config.InputFlags, kpis config.KPIs) ([]Task, error) {
	var tasks []Task

	if flags.PromKPIsConfig != "" {
		tasks = append(tasks, NewPromKPITask(kpis, flags))
	}

	// TODO: implement OslatTask and wire here
	if flags.OslatConfig != "" {
		return nil, fmt.Errorf("--oslat-config: task type not yet supported")
	}

	// TODO: implement PerNodeTask and wire here
	if flags.PerNodeConfig != "" {
		return nil, fmt.Errorf("--per-node-config: task type not yet supported")
	}

	// TODO: implement RecoveryTask and wire here
	if flags.RecoveryConfig != "" {
		return nil, fmt.Errorf("--recovery-config: task type not yet supported")
	}

	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks configured: at least one task config flag is required (e.g. --prom-kpis-config)")
	}

	return tasks, nil
}
