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

// ResolveFromFlags builds the task list for this run.
// Today only --kpis-file is supported (one Prom KPI task).
func ResolveFromFlags(flags config.InputFlags, kpis config.KPIs) ([]Task, error) {
	if flags.KPIsFile == "" {
		return nil, fmt.Errorf("no tasks configured: --kpis-file is required")
	}
	return []Task{NewPromKPITask(kpis, flags)}, nil
}
