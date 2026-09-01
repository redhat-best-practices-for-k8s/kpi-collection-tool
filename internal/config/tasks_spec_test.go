package config

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TasksSpec Loader", func() {
	var tmpDir string

	const validOslatSection = `
  timeout: 12h30m
  pod:
    metadata:
      name: oslat
    spec:
      containers:
        - name: oslat
          image: example.registry/oslat:latest`

	const validPerNodeSection = `
  workloadNamespaces:
    - nokia-workload
  duration: 30m
  interval: 5s
  nodes: all
  sshUser: core
  sshPort: 22
  remoteWorkDir: /home/core`

	const validRecoverySection = `
  workloadNamespaces:
    - nokia-workload
  duration: 30m
  interval: 1m
  startWhen: node-unreachable`

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "run-config-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	writeFile := func(name, content string) string {
		path := filepath.Join(tmpDir, name)
		Expect(os.WriteFile(path, []byte(content), 0644)).To(Succeed())
		return path
	}

	Describe("LoadTasksSpec", func() {
		Context("prometheus task config", func() {
			It("loads a prometheus task config with inline kpis", func() {
				path := writeFile("tasks.yaml", `
prometheus:
  kpis:
    - id: node-cpu
      promquery: node_cpu_seconds_total
`)
				cfg, err := LoadTasksSpec(path)

				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.Prometheus).NotTo(BeNil())
				Expect(cfg.Prometheus.Kpis).To(HaveLen(1))
				Expect(cfg.Prometheus.Kpis[0].ID).To(Equal("node-cpu"))
				Expect(cfg.Prometheus.ConfigFile).To(BeEmpty())
			})

			It("loads a prometheus task config with configFile", func() {
				path := writeFile("tasks.yaml", `
prometheus:
  configFile: prom-kpis.yaml
`)
				cfg, err := LoadTasksSpec(path)

				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.Prometheus).NotTo(BeNil())
				Expect(cfg.Prometheus.ConfigFile).To(Equal("prom-kpis.yaml"))
				Expect(cfg.Prometheus.Kpis).To(BeEmpty())
			})

			It("rejects configFile and kpis both set", func() {
				path := writeFile("tasks.yaml", `
prometheus:
  configFile: prom-kpis.yaml
  kpis:
    - id: node-cpu
      promquery: node_cpu_seconds_total
`)
				_, err := LoadTasksSpec(path)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("mutually exclusive"))
			})

			It("rejects an empty prometheus task config", func() {
				path := writeFile("tasks.yaml", `
prometheus: {}
`)
				_, err := LoadTasksSpec(path)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("one of configFile or kpis must be set"))
			})
		})

		Context("all task configs together", func() {
			It("loads all four task configs in default order", func() {
				path := writeFile("tasks.yaml", `
prometheus:
  kpis:
    - id: node-cpu
      promquery: node_cpu_seconds_total
per-node-data:`+validPerNodeSection+`
oslat:`+validOslatSection+`
app-recovery-time:`+validRecoverySection+`
`)
				cfg, err := LoadTasksSpec(path)

				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.PresentTaskConfigs()).To(Equal([]string{
					TaskConfigPrometheus, TaskConfigPerNodeData, TaskConfigOslat, TaskConfigAppRecoveryTime,
				}))
				Expect(cfg.Orchestration.Order).To(Equal(cfg.PresentTaskConfigs()))
			})
		})

		Context("file-level validation", func() {
			It("rejects a file with no task configs", func() {
				path := writeFile("tasks.yaml", `
orchestration:
  mode: sequential
`)
				_, err := LoadTasksSpec(path)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no task configs defined"))
			})

			It("returns an error for a missing file", func() {
				_, err := LoadTasksSpec(filepath.Join(tmpDir, "nonexistent.yaml"))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot access tasks path"))
			})

			It("returns an error for malformed YAML", func() {
				path := writeFile("tasks.yaml", `
prometheus:
  kpis: [invalid
`)
				_, err := LoadTasksSpec(path)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to decode tasks file"))
			})

			It("loads from a directory containing tasks.yaml", func() {
				writeFile("tasks.yaml", `
prometheus:
  kpis:
    - id: node-cpu
      promquery: node_cpu_seconds_total
`)
				cfg, err := LoadTasksSpec(tmpDir)

				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.Prometheus).NotTo(BeNil())
			})
		})

		Context("ResolvePath", func() {
			It("resolves a relative configFile against the tasks file's directory", func() {
				path := writeFile("tasks.yaml", `
prometheus:
  configFile: prom-kpis.yaml
`)
				cfg, err := LoadTasksSpec(path)

				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.ResolvePath(cfg.Prometheus.ConfigFile)).To(Equal(filepath.Join(tmpDir, "prom-kpis.yaml")))
			})

			It("leaves an absolute configFile path unchanged", func() {
				path := writeFile("tasks.yaml", `
prometheus:
  configFile: /abs/prom-kpis.yaml
`)
				cfg, err := LoadTasksSpec(path)

				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.ResolvePath(cfg.Prometheus.ConfigFile)).To(Equal("/abs/prom-kpis.yaml"))
			})
		})

		Context("orchestration defaults and validation", func() {
			It("defaults mode to sequential and on-failure to continue", func() {
				path := writeFile("tasks.yaml", `
prometheus:
  kpis:
    - id: node-cpu
      promquery: node_cpu_seconds_total
`)
				cfg, err := LoadTasksSpec(path)

				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.Orchestration.Mode).To(Equal(ModeSequential))
				Expect(cfg.Orchestration.OnFailure).To(Equal(OnFailureContinue))
				Expect(cfg.Orchestration.Order).To(Equal([]string{TaskConfigPrometheus}))
			})

			It("accepts explicit parallel mode and fail-fast on-failure", func() {
				path := writeFile("tasks.yaml", `
orchestration:
  mode: parallel
  on-failure: fail-fast
prometheus:
  kpis:
    - id: node-cpu
      promquery: node_cpu_seconds_total
`)
				cfg, err := LoadTasksSpec(path)

				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.Orchestration.Mode).To(Equal(ModeParallel))
				Expect(cfg.Orchestration.OnFailure).To(Equal(OnFailureFailFast))
			})

			It("rejects an invalid mode", func() {
				path := writeFile("tasks.yaml", `
orchestration:
  mode: banana
prometheus:
  kpis:
    - id: node-cpu
      promquery: node_cpu_seconds_total
`)
				_, err := LoadTasksSpec(path)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("orchestration.mode"))
			})

			It("rejects an invalid on-failure policy", func() {
				path := writeFile("tasks.yaml", `
orchestration:
  on-failure: maybe
prometheus:
  kpis:
    - id: node-cpu
      promquery: node_cpu_seconds_total
`)
				_, err := LoadTasksSpec(path)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("orchestration.on-failure"))
			})
		})

		Context("orchestration.order validation", func() {
			It("accepts an order that is a permutation of the present task configs", func() {
				path := writeFile("tasks.yaml", `
orchestration:
  order: [oslat, prometheus]
prometheus:
  kpis:
    - id: node-cpu
      promquery: node_cpu_seconds_total
oslat:`+validOslatSection+`
`)
				cfg, err := LoadTasksSpec(path)

				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.Orchestration.Order).To(Equal([]string{TaskConfigOslat, TaskConfigPrometheus}))
			})

			It("rejects an order with an unknown task config name", func() {
				path := writeFile("tasks.yaml", `
orchestration:
  order: [prometheus, not-a-real-section]
prometheus:
  kpis:
    - id: node-cpu
      promquery: node_cpu_seconds_total
`)
				_, err := LoadTasksSpec(path)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not a known task config"))
			})

			It("rejects an order that omits a present task config", func() {
				path := writeFile("tasks.yaml", `
orchestration:
  order: [prometheus]
prometheus:
  kpis:
    - id: node-cpu
      promquery: node_cpu_seconds_total
oslat:`+validOslatSection+`
`)
				_, err := LoadTasksSpec(path)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("missing task config(s)"))
			})

			It("rejects an order that lists a task config not present in the file", func() {
				path := writeFile("tasks.yaml", `
orchestration:
  order: [prometheus, oslat]
prometheus:
  kpis:
    - id: node-cpu
      promquery: node_cpu_seconds_total
`)
				_, err := LoadTasksSpec(path)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no corresponding task config"))
			})

			It("rejects a duplicate entry in order", func() {
				path := writeFile("tasks.yaml", `
orchestration:
  order: [prometheus, prometheus]
prometheus:
  kpis:
    - id: node-cpu
      promquery: node_cpu_seconds_total
`)
				_, err := LoadTasksSpec(path)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("listed more than once"))
			})
		})
	})

	Describe("TasksSpec.PresentTaskConfigs", func() {
		It("returns an empty slice when no task configs are set", func() {
			cfg := TasksSpec{}
			Expect(cfg.PresentTaskConfigs()).To(BeEmpty())
		})

		It("returns task configs in the fixed knownTaskConfigs order regardless of struct field order", func() {
			cfg := TasksSpec{
				AppRecoveryTime: &AppRecoveryTimeTaskConfig{WorkloadNamespaces: []string{"nokia-workload"}},
				Prometheus:      &PrometheusTaskConfig{Kpis: []Query{{ID: "x", PromQuery: "up"}}},
			}
			Expect(cfg.PresentTaskConfigs()).To(Equal([]string{TaskConfigPrometheus, TaskConfigAppRecoveryTime}))
		})
	})
})
