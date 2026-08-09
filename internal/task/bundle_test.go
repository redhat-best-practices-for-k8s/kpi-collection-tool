package task_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/task"
)

func defaultFlags() config.InputFlags {
	return config.InputFlags{}
}

func defaultKPIs() config.KPIs {
	return config.KPIs{}
}

var _ = Describe("Bundle", func() {

	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "bundle-test-*")
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

	Describe("LoadBundle", func() {
		It("loads a valid bundle file", func() {
			writeFile("bundle.yaml", `
tasks:
  - file: kpis.yaml
    type: prom-kpis
mode: sequential
on-failure: continue
`)
			spec, err := task.LoadBundle(filepath.Join(tmpDir, "bundle.yaml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Tasks).To(HaveLen(1))
			Expect(spec.Tasks[0].File).To(Equal("kpis.yaml"))
			Expect(spec.Tasks[0].Type).To(Equal("prom-kpis"))
			Expect(spec.Mode).To(Equal("sequential"))
			Expect(spec.OnFailure).To(Equal("continue"))
		})

		It("loads from a directory containing bundle.yaml", func() {
			writeFile("bundle.yaml", `
tasks:
  - file: kpis.yaml
    type: prom-kpis
`)
			spec, err := task.LoadBundle(tmpDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Tasks).To(HaveLen(1))
		})

		It("defaults mode to sequential and on-failure to continue", func() {
			writeFile("bundle.yaml", `
tasks:
  - file: kpis.yaml
    type: prom-kpis
`)
			spec, err := task.LoadBundle(filepath.Join(tmpDir, "bundle.yaml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Mode).To(Equal("sequential"))
			Expect(spec.OnFailure).To(Equal("continue"))
		})

		It("accepts parallel mode", func() {
			writeFile("bundle.yaml", `
tasks:
  - file: kpis.yaml
    type: prom-kpis
mode: parallel
on-failure: fail-fast
`)
			spec, err := task.LoadBundle(filepath.Join(tmpDir, "bundle.yaml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Mode).To(Equal("parallel"))
			Expect(spec.OnFailure).To(Equal("fail-fast"))
		})

		It("rejects an empty task list", func() {
			writeFile("bundle.yaml", `
tasks: []
`)
			_, err := task.LoadBundle(filepath.Join(tmpDir, "bundle.yaml"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no tasks defined"))
		})

		It("rejects an invalid mode", func() {
			writeFile("bundle.yaml", `
tasks:
  - file: kpis.yaml
mode: banana
`)
			_, err := task.LoadBundle(filepath.Join(tmpDir, "bundle.yaml"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid bundle mode"))
		})

		It("rejects an invalid on-failure policy", func() {
			writeFile("bundle.yaml", `
tasks:
  - file: kpis.yaml
on-failure: maybe
`)
			_, err := task.LoadBundle(filepath.Join(tmpDir, "bundle.yaml"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid bundle on-failure"))
		})

		It("rejects a task with no file", func() {
			writeFile("bundle.yaml", `
tasks:
  - type: prom-kpis
`)
			_, err := task.LoadBundle(filepath.Join(tmpDir, "bundle.yaml"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no file specified"))
		})

		It("returns error for missing path", func() {
			_, err := task.LoadBundle("/nonexistent/path")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot access"))
		})
	})

	Describe("inferTypeFromFilename", func() {
		It("handles unsupported task type in bundle", func() {
			writeFile("bundle.yaml", `
tasks:
  - file: custom.yaml
    type: custom-thing
`)
			writeFile("custom.yaml", "data: true")

			spec, err := task.LoadBundle(filepath.Join(tmpDir, "bundle.yaml"))
			Expect(err).NotTo(HaveOccurred())

			_, err = task.ResolveBundleTasks(spec, defaultFlags(), defaultKPIs())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not yet supported"))
		})
	})
})
