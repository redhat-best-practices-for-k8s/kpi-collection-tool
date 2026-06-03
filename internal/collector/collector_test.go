package collector

import (
	"database/sql"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/database"
)

// Helper function to create a Duration pointer
func durationPtr(d time.Duration) *config.Duration {
	return &config.Duration{Duration: d}
}

var _ = Describe("Collector", func() {
	Describe("groupKPIsByFrequency", func() {
		var (
			kpis        config.KPIs
			defaultFreq time.Duration
		)

		BeforeEach(func() {
			defaultFreq = 60 * time.Second
		})

		Context("when all KPIs have default frequency", func() {
			BeforeEach(func() {
				kpis = config.KPIs{
					Queries: []config.Query{
						{ID: "kpi-1", PromQuery: "query1"},
						{ID: "kpi-2", PromQuery: "query2"},
					},
				}
			})

			It("should group all KPIs under default frequency", func() {
				grouped := groupKPIsByFrequency(kpis, defaultFreq)

				Expect(grouped).To(HaveLen(1))
				Expect(grouped[60*time.Second].Queries).To(HaveLen(2))
				Expect(grouped[60*time.Second].Queries[0].ID).To(Equal("kpi-1"))
				Expect(grouped[60*time.Second].Queries[1].ID).To(Equal("kpi-2"))
			})
		})

		Context("when all KPIs have custom frequency", func() {
			BeforeEach(func() {
				kpis = config.KPIs{
					Queries: []config.Query{
						{ID: "kpi-1", PromQuery: "query1", SampleFrequency: durationPtr(10 * time.Second)},
						{ID: "kpi-2", PromQuery: "query2", SampleFrequency: durationPtr(30 * time.Second)},
					},
				}
			})

			It("should group KPIs by their custom frequencies", func() {
				grouped := groupKPIsByFrequency(kpis, defaultFreq)

				Expect(grouped).To(HaveLen(2))
				Expect(grouped[10*time.Second].Queries).To(HaveLen(1))
				Expect(grouped[10*time.Second].Queries[0].ID).To(Equal("kpi-1"))
				Expect(grouped[30*time.Second].Queries).To(HaveLen(1))
				Expect(grouped[30*time.Second].Queries[0].ID).To(Equal("kpi-2"))
			})
		})

		Context("when KPIs have mixed frequencies", func() {
			BeforeEach(func() {
				kpis = config.KPIs{
					Queries: []config.Query{
						{ID: "default-1", PromQuery: "query1"},
						{ID: "custom-1", PromQuery: "query2", SampleFrequency: durationPtr(15 * time.Second)},
						{ID: "default-2", PromQuery: "query3"},
					},
				}
			})

			It("should group KPIs correctly by frequency", func() {
				grouped := groupKPIsByFrequency(kpis, defaultFreq)

				Expect(grouped).To(HaveLen(2))
				Expect(grouped[60*time.Second].Queries).To(HaveLen(2))
				Expect(grouped[60*time.Second].Queries[0].ID).To(Equal("default-1"))
				Expect(grouped[60*time.Second].Queries[1].ID).To(Equal("default-2"))
				Expect(grouped[15*time.Second].Queries).To(HaveLen(1))
				Expect(grouped[15*time.Second].Queries[0].ID).To(Equal("custom-1"))
			})
		})

		Context("when there are no KPIs", func() {
			BeforeEach(func() {
				kpis = config.KPIs{Queries: []config.Query{}}
			})

			It("should return empty map", func() {
				grouped := groupKPIsByFrequency(kpis, defaultFreq)

				Expect(grouped).To(BeEmpty())
			})
		})

		Context("when multiple KPIs share the same custom frequency", func() {
			BeforeEach(func() {
				freq := durationPtr(30 * time.Second)
				kpis = config.KPIs{
					Queries: []config.Query{
						{ID: "kpi-1", PromQuery: "query1", SampleFrequency: freq},
						{ID: "kpi-2", PromQuery: "query2", SampleFrequency: freq},
						{ID: "kpi-3", PromQuery: "query3", SampleFrequency: freq},
					},
				}
			})

			It("should group all KPIs under the same frequency", func() {
				grouped := groupKPIsByFrequency(kpis, defaultFreq)

				Expect(grouped).To(HaveLen(1))
				Expect(grouped[30*time.Second].Queries).To(HaveLen(3))
				Expect(grouped[30*time.Second].Queries[0].ID).To(Equal("kpi-1"))
				Expect(grouped[30*time.Second].Queries[1].ID).To(Equal("kpi-2"))
				Expect(grouped[30*time.Second].Queries[2].ID).To(Equal("kpi-3"))
			})
		})
	})

	Describe("Query.GetEffectiveFrequency", func() {
		var defaultFreq time.Duration

		BeforeEach(func() {
			defaultFreq = 60 * time.Second
		})

		Context("when query has custom frequency", func() {
			It("should return the custom frequency", func() {
				query := config.Query{
					ID:              "test-kpi",
					PromQuery:       "test_query",
					SampleFrequency: durationPtr(30 * time.Second),
				}

				effectiveFreq := query.GetEffectiveFrequency(defaultFreq)
				Expect(effectiveFreq).To(Equal(30 * time.Second))
			})
		})

		Context("when query has no custom frequency", func() {
			It("should return the default frequency", func() {
				query := config.Query{
					ID:              "test-kpi",
					PromQuery:       "test_query",
					SampleFrequency: nil,
				}

				effectiveFreq := query.GetEffectiveFrequency(defaultFreq)
				Expect(effectiveFreq).To(Equal(60 * time.Second))
			})
		})

		Context("when query has zero custom frequency", func() {
			It("should return the default frequency", func() {
				query := config.Query{
					ID:              "test-kpi",
					PromQuery:       "test_query",
					SampleFrequency: durationPtr(0),
				}

				effectiveFreq := query.GetEffectiveFrequency(defaultFreq)
				Expect(effectiveFreq).To(Equal(60 * time.Second))
			})
		})

		Context("when query has negative custom frequency", func() {
			It("should return the default frequency", func() {
				query := config.Query{
					ID:              "test-kpi",
					PromQuery:       "test_query",
					SampleFrequency: durationPtr(-10 * time.Second),
				}

				effectiveFreq := query.GetEffectiveFrequency(defaultFreq)
				Expect(effectiveFreq).To(Equal(60 * time.Second))
			})
		})
	})

	Describe("startKPIGoroutines", func() {
		var (
			flags  config.InputFlags
			db     *sql.DB
			dbImpl database.Database
		)

		BeforeEach(func() {
			flags = config.InputFlags{
				SamplingFreq: 60 * time.Second,
				Duration:     1 * time.Second,
			}

			var err error
			db, dbImpl, err = database.InitDatabaseWithConfig("sqlite", "")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if db != nil {
				Expect(db.Close()).To(Succeed())
			}
		})

		Context("when there are KPIs with various frequencies", func() {
			It("should return cancel function and WaitGroup", func() {
				kpis := config.KPIs{
					Queries: []config.Query{
						{ID: "kpi-1", PromQuery: "query1", SampleFrequency: durationPtr(5 * time.Second)},
						{ID: "kpi-2", PromQuery: "query2"},
					},
				}

				var hadFailures atomic.Bool
				cancel, wg := startKPIGoroutines(db, dbImpl, kpis, flags, &hadFailures)

				Expect(cancel).NotTo(BeNil())
				Expect(wg).NotTo(BeNil())

				cancel()
				wg.Wait()
			})
		})

		Context("when there are only default frequency KPIs", func() {
			It("should start goroutines for default frequency", func() {
				kpis := config.KPIs{
					Queries: []config.Query{
						{ID: "kpi-1", PromQuery: "query1"},
						{ID: "kpi-2", PromQuery: "query2"},
					},
				}

				var hadFailures atomic.Bool
				cancel, wg := startKPIGoroutines(db, dbImpl, kpis, flags, &hadFailures)

				Expect(cancel).NotTo(BeNil())
				Expect(wg).NotTo(BeNil())

				cancel()
				wg.Wait()
			})
		})

		Context("when there are no KPIs", func() {
			It("should return cancel function and empty WaitGroup", func() {
				kpis := config.KPIs{Queries: []config.Query{}}

				var hadFailures atomic.Bool
				cancel, wg := startKPIGoroutines(db, dbImpl, kpis, flags, &hadFailures)

				Expect(cancel).NotTo(BeNil())
				Expect(wg).NotTo(BeNil())

				cancel()
				wg.Wait()
			})
		})
	})
})
