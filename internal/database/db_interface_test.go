package database

import (
	"database/sql"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/model"
)

// RunDatabaseInterfaceTests runs the complete test suite for any Database implementation.
// This ensures all implementations behave consistently according to the Database interface contract.
//
// Parameters:
//   - dbImpl: The Database implementation to test (SQLiteDB or PostgresDB)
//   - db: An active database connection
//
// Note: Uses Postgres-style parameter syntax ($1, $2) which is supported by both
// SQLite 3.32.0+ and PostgreSQL.
func RunDatabaseInterfaceTests(getImpl func() (Database, *sql.DB)) {
	var dbImpl Database
	var db *sql.DB

	BeforeEach(func() {
		dbImpl, db = getImpl()
	})
	Describe("GetOrCreateCluster", func() {
		Context("when cluster does not exist", func() {
			It("should create a new cluster and return its ID", func() {
				clusterID, err := dbImpl.GetOrCreateCluster(db, "test-cluster", "")
				Expect(err).NotTo(HaveOccurred())
				Expect(clusterID).To(BeNumerically(">", 0))

				var clusterName string
				err = db.QueryRow("SELECT cluster_name FROM clusters WHERE id = $1", clusterID).Scan(&clusterName)
				Expect(err).NotTo(HaveOccurred())
				Expect(clusterName).To(Equal("test-cluster"))
			})

			It("should set created_at timestamp", func() {
				clusterID, err := dbImpl.GetOrCreateCluster(db, "test-cluster", "")
				Expect(err).NotTo(HaveOccurred())

				var createdAt string
				err = db.QueryRow("SELECT created_at FROM clusters WHERE id = $1", clusterID).Scan(&createdAt)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdAt).NotTo(BeEmpty())
			})

			It("should create different IDs for different clusters", func() {
				clusterID1, err := dbImpl.GetOrCreateCluster(db, "cluster-1", "")
				Expect(err).NotTo(HaveOccurred())

				clusterID2, err := dbImpl.GetOrCreateCluster(db, "cluster-2", "")
				Expect(err).NotTo(HaveOccurred())

				Expect(clusterID1).NotTo(Equal(clusterID2))
			})
		})

		Context("when cluster already exists", func() {
			var existingClusterID int64

			BeforeEach(func() {
				var err error
				existingClusterID, err = dbImpl.GetOrCreateCluster(db, "existing-cluster", "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the existing cluster ID", func() {
				clusterID, err := dbImpl.GetOrCreateCluster(db, "existing-cluster", "")
				Expect(err).NotTo(HaveOccurred())
				Expect(clusterID).To(Equal(existingClusterID))
			})

			It("should not create a duplicate entry", func() {
				_, err := dbImpl.GetOrCreateCluster(db, "existing-cluster", "")
				Expect(err).NotTo(HaveOccurred())

				var count int
				err = db.QueryRow("SELECT COUNT(*) FROM clusters WHERE cluster_name = $1", "existing-cluster").Scan(&count)
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(Equal(1))
			})
		})
	})

	Describe("IncrementQueryError", func() {
		Context("when KPI has no previous errors", func() {
			It("should create a new error record with count 1", func() {
				err := dbImpl.IncrementQueryError(db, "test-kpi-1")
				Expect(err).NotTo(HaveOccurred())

				count, err := dbImpl.GetQueryErrorCount(db, "test-kpi-1")
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(Equal(1))
			})
		})

		Context("when KPI already has errors", func() {
			BeforeEach(func() {
				err := dbImpl.IncrementQueryError(db, "existing-kpi")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should increment the existing error count", func() {
				err := dbImpl.IncrementQueryError(db, "existing-kpi")
				Expect(err).NotTo(HaveOccurred())

				err = dbImpl.IncrementQueryError(db, "existing-kpi")
				Expect(err).NotTo(HaveOccurred())

				count, err := dbImpl.GetQueryErrorCount(db, "existing-kpi")
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(Equal(3))
			})
		})

		Context("with multiple different KPI IDs", func() {
			It("should handle them independently", func() {
				err := dbImpl.IncrementQueryError(db, "kpi-a")
				Expect(err).NotTo(HaveOccurred())
				err = dbImpl.IncrementQueryError(db, "kpi-a")
				Expect(err).NotTo(HaveOccurred())

				err = dbImpl.IncrementQueryError(db, "kpi-b")
				Expect(err).NotTo(HaveOccurred())

				countA, err := dbImpl.GetQueryErrorCount(db, "kpi-a")
				Expect(err).NotTo(HaveOccurred())
				Expect(countA).To(Equal(2))

				countB, err := dbImpl.GetQueryErrorCount(db, "kpi-b")
				Expect(err).NotTo(HaveOccurred())
				Expect(countB).To(Equal(1))
			})
		})
	})

	Describe("GetQueryErrorCount", func() {
		Context("when KPI ID does not exist", func() {
			It("should return 0", func() {
				count, err := dbImpl.GetQueryErrorCount(db, "non-existent-kpi")
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(Equal(0))
			})
		})

		Context("when KPI ID exists", func() {
			BeforeEach(func() {
				for i := 0; i < 5; i++ {
					err := dbImpl.IncrementQueryError(db, "test-kpi")
					Expect(err).NotTo(HaveOccurred())
				}
			})

			It("should return the correct count", func() {
				count, err := dbImpl.GetQueryErrorCount(db, "test-kpi")
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(Equal(5))
			})
		})
	})

	Describe("StoreQueryResults", func() {
		var clusterID int64

		BeforeEach(func() {
			var err error
			clusterID, err = dbImpl.GetOrCreateCluster(db, "test-cluster", "")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("with a single metric", func() {
			It("should store the metric value correctly", func() {
				vector := model.Vector{
					&model.Sample{
						Metric: model.Metric{
							"__name__": "test_metric",
							"label1":   "value1",
							"label2":   "value2",
						},
						Value:     model.SampleValue(42.5),
						Timestamp: model.Time(time.Now().Unix() * 1000),
					},
				}

				err := dbImpl.StoreQueryResults(db, clusterID, "test-query-1", "", vector)
				Expect(err).NotTo(HaveOccurred())

				var count int
				err = db.QueryRow("SELECT COUNT(*) FROM query_results WHERE kpi_id = $1", "test-query-1").Scan(&count)
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(Equal(1))

				var metricValue, timestampValue float64
				var metricLabels string
				var storedClusterID int64
				err = db.QueryRow(`
					SELECT metric_value, timestamp_value, cluster_id, metric_labels 
					FROM query_results 
					WHERE kpi_id = $1
				`, "test-query-1").Scan(&metricValue, &timestampValue, &storedClusterID, &metricLabels)
				Expect(err).NotTo(HaveOccurred())
				Expect(metricValue).To(Equal(42.5))
				Expect(storedClusterID).To(Equal(clusterID))
				Expect(metricLabels).To(ContainSubstring("test_metric"))
				Expect(metricLabels).To(ContainSubstring("label1"))
				Expect(metricLabels).To(ContainSubstring("value1"))
			})

			It("should store metric labels as JSON", func() {
				vector := model.Vector{
					&model.Sample{
						Metric: model.Metric{
							"__name__":  "cpu_usage",
							"namespace": "default",
							"pod":       "my-pod",
							"container": "my-container",
						},
						Value:     model.SampleValue(75.5),
						Timestamp: model.Time(time.Now().Unix() * 1000),
					},
				}
				err := dbImpl.StoreQueryResults(db, clusterID, "test-query-3", "", vector)
				Expect(err).NotTo(HaveOccurred())

				var metricLabels string
				err = db.QueryRow("SELECT metric_labels FROM query_results WHERE kpi_id = $1", "test-query-3").Scan(&metricLabels)
				Expect(err).NotTo(HaveOccurred())
				Expect(metricLabels).To(ContainSubstring("cpu_usage"))
				Expect(metricLabels).To(ContainSubstring("namespace"))
				Expect(metricLabels).To(ContainSubstring("default"))
				Expect(metricLabels).To(ContainSubstring("pod"))
				Expect(metricLabels).To(ContainSubstring("my-pod"))
			})

			It("should set execution_time and created_at timestamps", func() {
				vector := model.Vector{
					&model.Sample{
						Metric:    model.Metric{"__name__": "test"},
						Value:     model.SampleValue(1.0),
						Timestamp: model.Time(time.Now().Unix() * 1000),
					},
				}
				err := dbImpl.StoreQueryResults(db, clusterID, "test-query-4", "", vector)
				Expect(err).NotTo(HaveOccurred())

				var executionTime, createdAt string
				err = db.QueryRow(`
					SELECT execution_time, created_at 
					FROM query_results 
					WHERE kpi_id = $1
				`, "test-query-4").Scan(&executionTime, &createdAt)
				Expect(err).NotTo(HaveOccurred())
				Expect(executionTime).NotTo(BeEmpty())
				Expect(createdAt).NotTo(BeEmpty())
			})
		})

		Context("with multiple metrics", func() {
			It("should store all metrics from the vector", func() {
				vector := model.Vector{
					&model.Sample{
						Metric:    model.Metric{"__name__": "metric1"},
						Value:     model.SampleValue(10.0),
						Timestamp: model.Time(time.Now().Unix() * 1000),
					},
					&model.Sample{
						Metric:    model.Metric{"__name__": "metric2"},
						Value:     model.SampleValue(20.0),
						Timestamp: model.Time(time.Now().Unix() * 1000),
					},
					&model.Sample{
						Metric:    model.Metric{"__name__": "metric3"},
						Value:     model.SampleValue(30.0),
						Timestamp: model.Time(time.Now().Unix() * 1000),
					},
				}

				err := dbImpl.StoreQueryResults(db, clusterID, "test-query-2", "", vector)
				Expect(err).NotTo(HaveOccurred())

				var count int
				err = db.QueryRow("SELECT COUNT(*) FROM query_results WHERE kpi_id = $1", "test-query-2").Scan(&count)
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(Equal(3))
			})
		})

		Context("with multiple clusters", func() {
			var clusterID2 int64

			BeforeEach(func() {
				var err error
				clusterID2, err = dbImpl.GetOrCreateCluster(db, "another-cluster", "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should associate results with the correct cluster", func() {
				vector1 := model.Vector{
					&model.Sample{
						Metric:    model.Metric{"__name__": "metric"},
						Value:     model.SampleValue(1.0),
						Timestamp: model.Time(time.Now().Unix() * 1000),
					},
				}

				vector2 := model.Vector{
					&model.Sample{
						Metric:    model.Metric{"__name__": "metric"},
						Value:     model.SampleValue(2.0),
						Timestamp: model.Time(time.Now().Unix() * 1000),
					},
				}

				err := dbImpl.StoreQueryResults(db, clusterID, "query-cluster-1", "", vector1)
				Expect(err).NotTo(HaveOccurred())

				err = dbImpl.StoreQueryResults(db, clusterID2, "query-cluster-2", "", vector2)
				Expect(err).NotTo(HaveOccurred())

				var storedClusterID int64
				err = db.QueryRow("SELECT cluster_id FROM query_results WHERE kpi_id = $1", "query-cluster-1").Scan(&storedClusterID)
				Expect(err).NotTo(HaveOccurred())
				Expect(storedClusterID).To(Equal(clusterID))

				err = db.QueryRow("SELECT cluster_id FROM query_results WHERE kpi_id = $1", "query-cluster-2").Scan(&storedClusterID)
				Expect(err).NotTo(HaveOccurred())
				Expect(storedClusterID).To(Equal(clusterID2))
			})
		})
	})
}
