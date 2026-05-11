package commands

import (
	"database/sql"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/database"
	_ "modernc.org/sqlite"
)

func newInMemoryKPIDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}

	schema := `
	CREATE TABLE clusters (
		id INTEGER PRIMARY KEY,
		cluster_name TEXT NOT NULL
	);
	CREATE TABLE query_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		kpi_id TEXT NOT NULL,
		metric_value REAL,
		timestamp_value REAL,
		cluster_id INTEGER NOT NULL,
		execution_time TIMESTAMP,
		metric_labels TEXT
	);
	`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()

		return nil, err
	}

	return db, nil
}

var _ = Describe("queryKPIs", func() {
	var db *sql.DB

	BeforeEach(func() {
		var err error
		db, err = newInMemoryKPIDB()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if db != nil {
			_ = db.Close()
		}
	})

	Context("when filtering by --since", func() {
		It("should return only rows whose metric timestamp is after the since threshold", func() {
			now := time.Now()
			since := now.Add(-1 * time.Hour)

			_, err := db.Exec("INSERT INTO clusters (id, cluster_name) VALUES (?, ?)", 1, "cluster-a")
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(
				"INSERT INTO query_results (kpi_id, metric_value, timestamp_value, cluster_id, execution_time, metric_labels) VALUES (?, ?, ?, ?, ?, ?)",
				"kpi-since-hit", 10.0, float64(now.Add(-30*time.Minute).UnixNano())/1e9, 1, "2000-01-01 00:00:00", `{"instance":"a"}`,
			)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(
				"INSERT INTO query_results (kpi_id, metric_value, timestamp_value, cluster_id, execution_time, metric_labels) VALUES (?, ?, ?, ?, ?, ?)",
				"kpi-since-miss", 20.0, float64(now.Add(-2*time.Hour).UnixNano())/1e9, 1, "2099-01-01 00:00:00", `{"instance":"b"}`,
			)
			Expect(err).NotTo(HaveOccurred())

			results, err := queryKPIs(db, &database.SQLiteDB{}, KPIQueryParams{
				Since: &since,
				Sort:  "asc",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0].KPIName).To(Equal("kpi-since-hit"))
		})
	})

	Context("when filtering by --until", func() {
		It("should return only rows whose metric timestamp is before the until threshold", func() {
			now := time.Now()
			until := now.Add(-1 * time.Hour)

			_, err := db.Exec("INSERT INTO clusters (id, cluster_name) VALUES (?, ?)", 1, "cluster-a")
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(
				"INSERT INTO query_results (kpi_id, metric_value, timestamp_value, cluster_id, execution_time, metric_labels) VALUES (?, ?, ?, ?, ?, ?)",
				"kpi-until-hit", 10.0, float64(now.Add(-2*time.Hour).UnixNano())/1e9, 1, "2099-01-01 00:00:00", `{"instance":"a"}`,
			)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(
				"INSERT INTO query_results (kpi_id, metric_value, timestamp_value, cluster_id, execution_time, metric_labels) VALUES (?, ?, ?, ?, ?, ?)",
				"kpi-until-miss", 20.0, float64(now.Add(-30*time.Minute).UnixNano())/1e9, 1, "2000-01-01 00:00:00", `{"instance":"b"}`,
			)
			Expect(err).NotTo(HaveOccurred())

			results, err := queryKPIs(db, &database.SQLiteDB{}, KPIQueryParams{
				Until: &until,
				Sort:  "asc",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0].KPIName).To(Equal("kpi-until-hit"))
		})
	})
})

var _ = Describe("parseTimeFilter", func() {
	var now time.Time

	BeforeEach(func() {
		now = time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	})

	It("should accept a Go duration string and return now minus that duration", func() {
		got, err := parseTimeFilter("2h", now)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(BeTemporally("~", now.Add(-2*time.Hour)))
	})

	It("should accept an RFC 3339 timestamp in UTC", func() {
		input := "2026-04-07T12:24:25Z"
		got, err := parseTimeFilter(input, now)
		Expect(err).NotTo(HaveOccurred())

		want, err := time.Parse(time.RFC3339, input)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(BeTemporally("~", want))
	})

	It("should accept an RFC 3339 timestamp with timezone offset", func() {
		input := "2026-04-07T12:24:25+02:00"
		got, err := parseTimeFilter(input, now)
		Expect(err).NotTo(HaveOccurred())

		want, err := time.Parse(time.RFC3339, input)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(BeTemporally("~", want))
	})

	It("should reject a timestamp that is not a valid duration nor RFC 3339", func() {
		_, err := parseTimeFilter("2026-04-07-12:24:25", now)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("must be a Go duration"))
	})

	It("should reject a non-positive duration", func() {
		_, err := parseTimeFilter("0s", now)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("must be > 0"))
	})
})

var _ = Describe("parseKPIQueryTimeWindow", func() {
	var now time.Time

	BeforeEach(func() {
		now = time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	})

	It("should reject when --since resolves after --until", func() {
		_, _, err := parseKPIQueryTimeWindow("1h", "2h", now)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("must resolve before --until"))
	})
})
