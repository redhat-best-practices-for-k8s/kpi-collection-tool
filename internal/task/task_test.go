package task_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/task"
)

var _ = Describe("PromKPITask", func() {
	It("uses the prometheus task-config name", func() {
		t := task.NewPromKPITask(config.KPIs{}, config.InputFlags{})
		Expect(t.Name()).To(Equal(config.TaskConfigPrometheus))
	})
})

var _ = Describe("FromPromKPIsFlag", func() {
	It("returns one prometheus task when --prom-kpis-config is set", func() {
		flags := config.InputFlags{PromKPIsConfig: "kpis.yaml"}
		kpis := config.KPIs{Queries: []config.Query{{ID: "cpu", PromQuery: "up"}}}

		tasks, err := task.FromPromKPIsFlag(flags, kpis)
		Expect(err).NotTo(HaveOccurred())
		Expect(tasks).To(HaveLen(1))
		Expect(tasks[0].Name()).To(Equal(config.TaskConfigPrometheus))
	})

	It("returns an error when --prom-kpis-config is not set", func() {
		flags := config.InputFlags{}
		kpis := config.KPIs{}

		tasks, err := task.FromPromKPIsFlag(flags, kpis)
		Expect(err).To(HaveOccurred())
		Expect(tasks).To(BeNil())
		Expect(err.Error()).To(ContainSubstring("--prom-kpis-config is required"))
	})
})

var _ = Describe("unimplemented task stubs", func() {
	It("oslat is named and not yet supported", func() {
		t := task.NewOslatTask(config.InputFlags{})
		Expect(t.Name()).To(Equal(config.TaskConfigOslat))
		Expect(t.Run(context.Background())).To(MatchError(ContainSubstring("not yet supported")))
	})

	It("per-node-data is named and not yet supported", func() {
		t := task.NewPerNodeDataTask(config.InputFlags{})
		Expect(t.Name()).To(Equal(config.TaskConfigPerNodeData))
		Expect(t.Run(context.Background())).To(MatchError(ContainSubstring("not yet supported")))
	})

	It("app-recovery-time is named and not yet supported", func() {
		t := task.NewAppRecoveryTimeTask(config.InputFlags{})
		Expect(t.Name()).To(Equal(config.TaskConfigAppRecoveryTime))
		Expect(t.Run(context.Background())).To(MatchError(ContainSubstring("not yet supported")))
	})
})

var _ = Describe("ResolveFromTasksSpec", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "run-config-resolve-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	flags := config.InputFlags{ClusterName: "test-cluster"}

	It("builds a prometheus task from inline kpis", func() {
		cfg := config.TasksSpec{
			Prometheus: &config.PrometheusTaskConfig{
				Kpis: []config.Query{{ID: "node-cpu", PromQuery: "up"}},
			},
		}

		tasks, err := task.ResolveFromTasksSpec(cfg, flags)
		Expect(err).NotTo(HaveOccurred())
		Expect(tasks).To(HaveLen(1))
		Expect(tasks[0].Name()).To(Equal(config.TaskConfigPrometheus))
	})

	It("builds a prometheus task from a configFile", func() {
		kpisPath := filepath.Join(tmpDir, "prom-kpis.yaml")
		Expect(os.WriteFile(kpisPath, []byte(`
kpis:
  - id: node-cpu
    promquery: up
`), 0644)).To(Succeed())

		cfg := config.TasksSpec{
			Prometheus: &config.PrometheusTaskConfig{ConfigFile: kpisPath},
		}

		tasks, err := task.ResolveFromTasksSpec(cfg, flags)
		Expect(err).NotTo(HaveOccurred())
		Expect(tasks).To(HaveLen(1))
		Expect(tasks[0].Name()).To(Equal(config.TaskConfigPrometheus))
	})

	It("resolves a relative configFile against the tasks file directory", func() {
		Expect(os.WriteFile(filepath.Join(tmpDir, "prom-kpis.yaml"), []byte(`
kpis:
  - id: node-cpu
    promquery: up
`), 0644)).To(Succeed())
		tasksPath := filepath.Join(tmpDir, "tasks.yaml")
		Expect(os.WriteFile(tasksPath, []byte(`
prometheus:
  configFile: prom-kpis.yaml
`), 0644)).To(Succeed())

		cfg, err := config.LoadTasksSpec(tasksPath)
		Expect(err).NotTo(HaveOccurred())

		tasks, err := task.ResolveFromTasksSpec(cfg, flags)
		Expect(err).NotTo(HaveOccurred())
		Expect(tasks).To(HaveLen(1))
		Expect(tasks[0].Name()).To(Equal(config.TaskConfigPrometheus))
	})

	It("returns not-yet-supported for oslat", func() {
		cfg := config.TasksSpec{
			Oslat: map[string]interface{}{},
		}

		tasks, err := task.ResolveFromTasksSpec(cfg, flags)
		Expect(err).To(HaveOccurred())
		Expect(tasks).To(BeNil())
		Expect(err.Error()).To(ContainSubstring(config.TaskConfigOslat))
		Expect(err.Error()).To(ContainSubstring("not yet supported"))
	})

	It("returns not-yet-supported for per-node-data", func() {
		cfg := config.TasksSpec{
			PerNodeData: map[string]interface{}{},
		}

		tasks, err := task.ResolveFromTasksSpec(cfg, flags)
		Expect(err).To(HaveOccurred())
		Expect(tasks).To(BeNil())
		Expect(err.Error()).To(ContainSubstring(config.TaskConfigPerNodeData))
		Expect(err.Error()).To(ContainSubstring("not yet supported"))
	})

	It("returns not-yet-supported for app-recovery-time", func() {
		cfg := config.TasksSpec{
			AppRecoveryTime: map[string]interface{}{},
		}

		tasks, err := task.ResolveFromTasksSpec(cfg, flags)
		Expect(err).To(HaveOccurred())
		Expect(tasks).To(BeNil())
		Expect(err.Error()).To(ContainSubstring(config.TaskConfigAppRecoveryTime))
		Expect(err.Error()).To(ContainSubstring("not yet supported"))
	})

	It("uses default order so the first unsupported task is the one that fails", func() {
		cfg := config.TasksSpec{
			Oslat:           map[string]interface{}{},
			AppRecoveryTime: map[string]interface{}{},
		}

		_, err := task.ResolveFromTasksSpec(cfg, flags)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(config.TaskConfigOslat))
	})

	It("honors orchestration.order when choosing which unsupported task fails first", func() {
		cfg := config.TasksSpec{
			Orchestration: config.OrchestrationConfig{
				Order: []string{config.TaskConfigAppRecoveryTime, config.TaskConfigOslat},
			},
			Oslat:           map[string]interface{}{},
			AppRecoveryTime: map[string]interface{}{},
		}

		_, err := task.ResolveFromTasksSpec(cfg, flags)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(config.TaskConfigAppRecoveryTime))
	})

	It("returns an error when no task configs are present", func() {
		tasks, err := task.ResolveFromTasksSpec(config.TasksSpec{}, flags)
		Expect(err).To(HaveOccurred())
		Expect(tasks).To(BeNil())
		Expect(err.Error()).To(ContainSubstring("no task configs defined"))
	})
})
