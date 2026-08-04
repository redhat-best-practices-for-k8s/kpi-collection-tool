package task_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/task"
)

var _ = Describe("ResolveFromFlags", func() {
	It("returns one prom-kpis task when --kpis-file is set", func() {
		flags := config.InputFlags{KPIsFile: "kpis.yaml"}
		kpis := config.KPIs{Queries: []config.Query{{ID: "cpu", PromQuery: "up"}}}

		tasks, err := task.ResolveFromFlags(flags, kpis)
		Expect(err).NotTo(HaveOccurred())
		Expect(tasks).To(HaveLen(1))
		Expect(tasks[0].Name()).To(Equal("prom-kpis"))
	})

	It("returns an error when no --kpis-file is set", func() {
		flags := config.InputFlags{}
		kpis := config.KPIs{}

		tasks, err := task.ResolveFromFlags(flags, kpis)
		Expect(err).To(HaveOccurred())
		Expect(tasks).To(BeNil())
		Expect(err.Error()).To(ContainSubstring("--kpis-file"))
	})
})
