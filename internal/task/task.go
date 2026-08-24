// Package task defines runnable units executed by `kpi-collector run`.
package task

import (
	"context"
	"fmt"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/collector"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
)

// Task is one unit of work `run` can execute.
type Task interface {
	Name() string
	Run(ctx context.Context) error
}

func errNotYetSupported(name string) error {
	return fmt.Errorf("%s: task type not yet supported", name)
}

// PromKPITask collects Prometheus/Thanos KPI metrics.
type PromKPITask struct {
	kpis  config.KPIs
	flags config.InputFlags
}

// NewPromKPITask creates a Prometheus collection task.
func NewPromKPITask(kpis config.KPIs, flags config.InputFlags) *PromKPITask {
	return &PromKPITask{kpis: kpis, flags: flags}
}

func (t *PromKPITask) Name() string {
	return config.TaskConfigPrometheus
}

func (t *PromKPITask) Run(ctx context.Context) error {
	_ = ctx

	if t.flags.SingleRun {
		return collector.RunKPIsOnce(t.kpis, t.flags)
	}
	return collector.RunKPIs(t.kpis, t.flags)
}

// OslatTask is a stub for the oslat task (--tasks oslat section).
type OslatTask struct {
	flags config.InputFlags
}

// NewOslatTask creates an oslat stub. Fill in Run when the schema is implemented.
func NewOslatTask(flags config.InputFlags) *OslatTask {
	return &OslatTask{flags: flags}
}

func (t *OslatTask) Name() string { return config.TaskConfigOslat }

func (t *OslatTask) Run(ctx context.Context) error {
	_ = ctx
	return errNotYetSupported(t.Name())
}

// PerNodeDataTask is a stub for the per-node-data task.
type PerNodeDataTask struct {
	flags config.InputFlags
}

// NewPerNodeDataTask creates a per-node-data stub. Fill in Run when the schema is implemented.
func NewPerNodeDataTask(flags config.InputFlags) *PerNodeDataTask {
	return &PerNodeDataTask{flags: flags}
}

func (t *PerNodeDataTask) Name() string { return config.TaskConfigPerNodeData }

func (t *PerNodeDataTask) Run(ctx context.Context) error {
	_ = ctx
	return errNotYetSupported(t.Name())
}

// AppRecoveryTimeTask is a stub for the app-recovery-time task.
type AppRecoveryTimeTask struct {
	flags config.InputFlags
}

// NewAppRecoveryTimeTask creates an app-recovery-time stub. Fill in Run when the schema is implemented.
func NewAppRecoveryTimeTask(flags config.InputFlags) *AppRecoveryTimeTask {
	return &AppRecoveryTimeTask{flags: flags}
}

func (t *AppRecoveryTimeTask) Name() string { return config.TaskConfigAppRecoveryTime }

func (t *AppRecoveryTimeTask) Run(ctx context.Context) error {
	_ = ctx
	return errNotYetSupported(t.Name())
}
