package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/model"
)

var _ = Describe("Sqlite", func() {
	var (
		db        *sql.DB
		tmpDir    string
		sqliteDB  *SQLiteDB
		originCwd string
	)

	BeforeEach(func() {
		sqliteDB = NewSQLiteDB()
		OutputDir = DefaultOutputDir

		var err error
		tmpDir, err = os.MkdirTemp("", "sqlite-test-*")
		Expect(err).NotTo(HaveOccurred())

		// Change to temp directory so InitDB creates the artifact dir there
		originCwd, err = os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		err = os.Chdir(tmpDir)
		Expect(err).NotTo(HaveOccurred())

		db, err = sqliteDB.InitDB()
		Expect(err).NotTo(HaveOccurred())
		Expect(db).NotTo(BeNil())
	})

	AfterEach(func() {
		if db != nil {
			err := db.Close()
			Expect(err).NotTo(HaveOccurred())
		}
		OutputDir = DefaultOutputDir
		if originCwd != "" {
			err := os.Chdir(originCwd)
			Expect(err).NotTo(HaveOccurred())
		}
		if tmpDir != "" {
			err := os.RemoveAll(tmpDir)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Describe("SQLite-Specific Features", func() {
		It("should create the database and required tables", func() {
			var tableName string
			err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='clusters'").Scan(&tableName)
			Expect(err).NotTo(HaveOccurred())
			Expect(tableName).To(Equal("clusters"))

			err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='query_results'").Scan(&tableName)
			Expect(err).NotTo(HaveOccurred())
			Expect(tableName).To(Equal("query_results"))

			err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='query_errors'").Scan(&tableName)
			Expect(err).NotTo(HaveOccurred())
			Expect(tableName).To(Equal("query_errors"))
		})

		It("should create the data directory", func() {
			dataDir := filepath.Join(tmpDir, DefaultOutputDir)
			_, err := os.Stat(dataDir)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create the database file", func() {
			dbFile := filepath.Join(tmpDir, DefaultOutputDir, DefaultDBFileName)
			_, err := os.Stat(dbFile)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Category Tables", func() {
		var clusterID int64

		BeforeEach(func() {
			var err error
			clusterID, err = sqliteDB.GetOrCreateCluster(db, "cat-cluster", "ran")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create the kpi_registry table at init", func() {
			var name string
			err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='kpi_registry'").Scan(&name)
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal("kpi_registry"))
		})

		It("should create a category table and register the KPI", func() {
			tableName, err := sqliteDB.EnsureCategoryTable(db, "cpu", "node-cpu-usage")
			Expect(err).NotTo(HaveOccurred())
			Expect(tableName).To(Equal("kpi_cpu"))

			var name string
			err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='kpi_cpu'").Scan(&name)
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal("kpi_cpu"))

			var kpiID, category, regTable string
			err = db.QueryRow("SELECT kpi_id, category, table_name FROM kpi_registry WHERE kpi_id = 'node-cpu-usage'").
				Scan(&kpiID, &category, &regTable)
			Expect(err).NotTo(HaveOccurred())
			Expect(category).To(Equal("cpu"))
			Expect(regTable).To(Equal("kpi_cpu"))
		})

		It("should be idempotent for the same category", func() {
			_, err := sqliteDB.EnsureCategoryTable(db, "memory", "mem-usage-1")
			Expect(err).NotTo(HaveOccurred())
			_, err = sqliteDB.EnsureCategoryTable(db, "memory", "mem-usage-2")
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM kpi_registry WHERE category = 'memory'").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(2))
		})

		It("should route categorised writes to the category table", func() {
			vector := model.Vector{
				&model.Sample{
					Metric:    model.Metric{"__name__": "cpu_seconds"},
					Value:     model.SampleValue(0.75),
					Timestamp: model.Time(time.Now().Unix() * 1000),
				},
			}

			err := sqliteDB.StoreQueryResults(db, clusterID, "node-cpu", "cpu", vector)
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM kpi_cpu WHERE kpi_id = 'node-cpu'").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))

			err = db.QueryRow("SELECT COUNT(*) FROM query_results WHERE kpi_id = 'node-cpu'").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("should route uncategorised writes to query_results", func() {
			vector := model.Vector{
				&model.Sample{
					Metric:    model.Metric{"__name__": "up"},
					Value:     model.SampleValue(1),
					Timestamp: model.Time(time.Now().Unix() * 1000),
				},
			}

			err := sqliteDB.StoreQueryResults(db, clusterID, "cluster-up", "", vector)
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM query_results WHERE kpi_id = 'cluster-up'").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("should enforce dedup on category tables", func() {
			ts := model.Time(time.Now().Unix() * 1000)
			vector := model.Vector{
				&model.Sample{
					Metric:    model.Metric{"__name__": "mem_bytes"},
					Value:     model.SampleValue(1024),
					Timestamp: ts,
				},
			}

			err := sqliteDB.StoreQueryResults(db, clusterID, "mem-kpi", "memory", vector)
			Expect(err).NotTo(HaveOccurred())
			err = sqliteDB.StoreQueryResults(db, clusterID, "mem-kpi", "memory", vector)
			Expect(err).NotTo(HaveOccurred())

			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM kpi_memory WHERE kpi_id = 'mem-kpi'").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})
	})

	RunDatabaseInterfaceTests(func() (Database, *sql.DB) { return sqliteDB, db })
})
