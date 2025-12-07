package database

import (
	"database/sql"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sqlite", func() {
	var (
		db           *sql.DB
		tmpDir       string
		sqliteDB     *SQLiteDB
		originalHome string
	)

	// Runs before and after each test (It section)
	// To provide clean, isolated environment for each test
	BeforeEach(func() {
		// Create SQLiteDB instance
		sqliteDB = NewSQLiteDB()

		// Create a temporary directory to act as HOME for test isolation
		var err error
		tmpDir, err = os.MkdirTemp("", "sqlite-test-*")
		Expect(err).NotTo(HaveOccurred())

		// Override HOME environment variable so GetSQLiteDBPath() uses our temp dir
		originalHome = os.Getenv("HOME")
		err = os.Setenv("HOME", tmpDir)
		Expect(err).NotTo(HaveOccurred())

		// Initialize database (will create in tmpDir/.kpi-collector/)
		db, err = sqliteDB.InitDB()
		Expect(err).NotTo(HaveOccurred())
		Expect(db).NotTo(BeNil())
	})

	AfterEach(func() {
		if db != nil {
			err := db.Close()
			Expect(err).NotTo(HaveOccurred())
		}
		// Restore original HOME
		if originalHome != "" {
			err := os.Setenv("HOME", originalHome)
			Expect(err).NotTo(HaveOccurred())
		}
		// Clean up temporary directory
		if tmpDir != "" {
			err := os.RemoveAll(tmpDir)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Describe("SQLite-Specific Features", func() {
		It("should create the database and required tables", func() {
			// sqlite_master = special system table in SQLite that contains
			// metadata about all the database objects (tables, indexes, views, triggers)
			// in your SQLite database.
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
			dataDir := filepath.Join(tmpDir, ".kpi-collector")
			_, err := os.Stat(dataDir)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create the database file", func() {
			dbFile := filepath.Join(tmpDir, ".kpi-collector", "kpi_metrics.db")
			_, err := os.Stat(dbFile)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	// Run all the shared interface tests
	RunDatabaseInterfaceTests(func() (Database, *sql.DB) { return sqliteDB, db })
})
