package commands

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/database"
	_ "modernc.org/sqlite"
)

func TestQueryKPIsSinceFiltersByMetricTimestamp(t *testing.T) {
	db := newInMemoryKPIDB(t)
	defer func() { _ = db.Close() }()

	now := time.Now()
	since := now.Add(-1 * time.Hour)

	mustExec(t, db, "INSERT INTO clusters (id, cluster_name) VALUES (?, ?)", 1, "cluster-a")
	mustExec(t, db,
		"INSERT INTO query_results (kpi_id, metric_value, timestamp_value, cluster_id, execution_time, metric_labels) VALUES (?, ?, ?, ?, ?, ?)",
		"kpi-since-hit", 10.0, float64(now.Add(-30*time.Minute).UnixNano())/1e9, 1, "2000-01-01 00:00:00", `{"instance":"a"}`,
	)
	mustExec(t, db,
		"INSERT INTO query_results (kpi_id, metric_value, timestamp_value, cluster_id, execution_time, metric_labels) VALUES (?, ?, ?, ?, ?, ?)",
		"kpi-since-miss", 20.0, float64(now.Add(-2*time.Hour).UnixNano())/1e9, 1, "2099-01-01 00:00:00", `{"instance":"b"}`,
	)

	results, err := queryKPIs(db, &database.SQLiteDB{}, KPIQueryParams{
		Since: &since,
		Sort:  "asc",
	})
	if err != nil {
		t.Fatalf("queryKPIs returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].KPIName != "kpi-since-hit" {
		t.Fatalf("expected kpi-since-hit, got %s", results[0].KPIName)
	}
}

func TestQueryKPIsUntilFiltersByMetricTimestamp(t *testing.T) {
	db := newInMemoryKPIDB(t)
	defer func() { _ = db.Close() }()

	now := time.Now()
	until := now.Add(-1 * time.Hour)

	mustExec(t, db, "INSERT INTO clusters (id, cluster_name) VALUES (?, ?)", 1, "cluster-a")
	mustExec(t, db,
		"INSERT INTO query_results (kpi_id, metric_value, timestamp_value, cluster_id, execution_time, metric_labels) VALUES (?, ?, ?, ?, ?, ?)",
		"kpi-until-hit", 10.0, float64(now.Add(-2*time.Hour).UnixNano())/1e9, 1, "2099-01-01 00:00:00", `{"instance":"a"}`,
	)
	mustExec(t, db,
		"INSERT INTO query_results (kpi_id, metric_value, timestamp_value, cluster_id, execution_time, metric_labels) VALUES (?, ?, ?, ?, ?, ?)",
		"kpi-until-miss", 20.0, float64(now.Add(-30*time.Minute).UnixNano())/1e9, 1, "2000-01-01 00:00:00", `{"instance":"b"}`,
	)

	results, err := queryKPIs(db, &database.SQLiteDB{}, KPIQueryParams{
		Until: &until,
		Sort:  "asc",
	})
	if err != nil {
		t.Fatalf("queryKPIs returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].KPIName != "kpi-until-hit" {
		t.Fatalf("expected kpi-until-hit, got %s", results[0].KPIName)
	}
}

func newInMemoryKPIDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
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
		t.Fatalf("failed to initialize schema: %v", err)
	}

	return db
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...interface{}) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec failed for query %q: %v", query, err)
	}
}

func TestParseTimeFilterAcceptsDuration(t *testing.T) {
	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)

	got, err := parseTimeFilter("2h", now)
	if err != nil {
		t.Fatalf("parseTimeFilter returned error: %v", err)
	}

	want := now.Add(-2 * time.Hour)
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want.Format(time.RFC3339), got.Format(time.RFC3339))
	}
}

func TestParseTimeFilterAcceptsRFC3339(t *testing.T) {
	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
	input := "2026-04-07T12:24:25Z"

	got, err := parseTimeFilter(input, now)
	if err != nil {
		t.Fatalf("parseTimeFilter returned error: %v", err)
	}

	want, err := time.Parse(time.RFC3339, input)
	if err != nil {
		t.Fatalf("failed to parse expected RFC3339 value: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want.Format(time.RFC3339), got.Format(time.RFC3339))
	}
}

func TestParseTimeFilterRejectsNonPositiveDuration(t *testing.T) {
	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)

	_, err := parseTimeFilter("0s", now)
	if err == nil {
		t.Fatalf("expected error for non-positive duration")
	}
	if !strings.Contains(err.Error(), "must be > 0") {
		t.Fatalf("expected positive-duration validation error, got: %v", err)
	}
}

func TestParseKPIQueryTimeWindowRejectsInvalidOrder(t *testing.T) {
	now := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)

	_, _, err := parseKPIQueryTimeWindow("1h", "2h", now)
	if err == nil {
		t.Fatalf("expected error when --since resolves after --until")
	}
	if !strings.Contains(err.Error(), "must resolve before --until") {
		t.Fatalf("expected since-before-until validation error, got: %v", err)
	}
}
