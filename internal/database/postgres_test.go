package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	postgresContainer *postgres.PostgresContainer
	postgresURL       string
)

// BeforeSuite runs once before all Postgres tests
// It starts a Postgres Docker container for testing
var _ = BeforeSuite(func() {
	ctx := context.Background()

	// Start Postgres container
	var err error
	postgresContainer, err = postgres.Run(ctx,
		"postgres:15-alpine", // Lightweight Alpine image
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	Expect(err).NotTo(HaveOccurred(), "Failed to start Postgres container. Is Docker running?")

	// Get connection string
	postgresURL, err = postgresContainer.ConnectionString(ctx, "sslmode=disable")
	Expect(err).NotTo(HaveOccurred())

})

// AfterSuite runs once after all Postgres tests
// It tears down the Postgres container
var _ = AfterSuite(func() {
	if postgresContainer != nil {
		ctx := context.Background()
		err := postgresContainer.Terminate(ctx)
		Expect(err).NotTo(HaveOccurred())
	}
})

var _ = Describe("Postgres Implementation", func() {
	var (
		db         *sql.DB
		postgresDB *PostgresDB
	)

	BeforeEach(func() {
		postgresDB = NewPostgresDB(postgresURL)
		var err error
		db, err = postgresDB.InitDB()
		Expect(err).NotTo(HaveOccurred())

		dropPostgresCategoryTables(db)
		_, err = db.Exec("TRUNCATE TABLE kpi_registry, query_results, query_errors, clusters RESTART IDENTITY CASCADE")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if db != nil {
			dropPostgresCategoryTables(db)
			_, _ = db.Exec("TRUNCATE TABLE kpi_registry, query_results, query_errors, clusters RESTART IDENTITY CASCADE")
			_ = db.Close()
		}
	})

	Describe("Postgres-Specific Features", func() {
		It("should create the database and required tables", func() {
			// Verifies our InitDB() successfully created all tables
			// information_schema = standard SQL schema containing database metadata
			var tableName string
			err := db.QueryRow("SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name='clusters'").Scan(&tableName)
			Expect(err).NotTo(HaveOccurred())
			Expect(tableName).To(Equal("clusters"))

			err = db.QueryRow("SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name='query_results'").Scan(&tableName)
			Expect(err).NotTo(HaveOccurred())
			Expect(tableName).To(Equal("query_results"))

			err = db.QueryRow("SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name='query_errors'").Scan(&tableName)
			Expect(err).NotTo(HaveOccurred())
			Expect(tableName).To(Equal("query_errors"))
		})

		It("should use JSONB data type for metric_labels", func() {
			// Tests that our InitDB() creates metric_labels as JSONB type
			// This verifies our schema definition is correct

			var dataType string
			err := db.QueryRow(`
				SELECT data_type 
				FROM information_schema.columns 
				WHERE table_name = 'query_results' 
				AND column_name = 'metric_labels'
			`).Scan(&dataType)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataType).To(Equal("jsonb"))
		})

		It("should have GIN index on JSONB column", func() {
			// Tests that our InitDB() creates the GIN index on metric_labels
			// This verifies our index creation statement executed correctly

			var indexExists bool
			err := db.QueryRow(`
				SELECT EXISTS (
					SELECT 1 FROM pg_indexes 
					WHERE indexname = 'idx_query_results_labels'
				)
			`).Scan(&indexExists)
			Expect(err).NotTo(HaveOccurred())
			Expect(indexExists).To(BeTrue(), "GIN index should exist for JSONB queries")
		})

	})

	Describe("Postgres Category Tables", func() {
		It("should create the kpi_registry table at init", func() {
			var tableName string
			err := db.QueryRow(
				"SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name='kpi_registry'",
			).Scan(&tableName)
			Expect(err).NotTo(HaveOccurred())
			Expect(tableName).To(Equal("kpi_registry"))
		})

		It("should create a category table with JSONB column", func() {
			_, err := postgresDB.EnsureCategoryTable(db, "cpu", "node-cpu")
			Expect(err).NotTo(HaveOccurred())

			var dataType string
			err = db.QueryRow(`
				SELECT data_type FROM information_schema.columns
				WHERE table_name = 'kpi_cpu' AND column_name = 'metric_labels'
			`).Scan(&dataType)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataType).To(Equal("jsonb"))
		})

		It("should create a dedup index on the category table", func() {
			_, err := postgresDB.EnsureCategoryTable(db, "network", "net-kpi")
			Expect(err).NotTo(HaveOccurred())

			var indexExists bool
			err = db.QueryRow(`
				SELECT EXISTS (
					SELECT 1 FROM pg_indexes WHERE indexname = 'idx_kpi_network_dedup'
				)
			`).Scan(&indexExists)
			Expect(err).NotTo(HaveOccurred())
			Expect(indexExists).To(BeTrue())
		})
	})

	// Run all the shared interface tests
	RunDatabaseInterfaceTests(func() (Database, *sql.DB) { return postgresDB, db })
})

func dropPostgresCategoryTables(db *sql.DB) {
	rows, err := db.Query(`
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_name LIKE 'kpi_%'
		AND table_name != 'kpi_registry'`)
	if err != nil {
		return
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var tn string
		if rows.Scan(&tn) == nil {
			_, _ = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", tn))
		}
	}
	_ = rows.Err()
}
