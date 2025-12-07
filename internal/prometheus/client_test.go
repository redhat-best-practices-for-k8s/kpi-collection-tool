package prometheus

import (
	"context"
	"database/sql"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"kpi-collector/internal/database"
	"kpi-collector/internal/output"
)

// mockPromAPI is a simple mock that implements only the Query method we actually use.
// All other methods return empty/nil values to satisfy the v1.API interface.
type mockPromAPI struct {
	queryFunc func(ctx context.Context, query string, ts time.Time) (model.Value, v1.Warnings, error)
}

func (m *mockPromAPI) Query(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {
	if m.queryFunc != nil {
		return m.queryFunc(ctx, query, ts)
	}
	return nil, nil, nil
}

// All methods below are unused but required to implement v1.API interface
func (m *mockPromAPI) QueryRange(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (model.Value, v1.Warnings, error) {
	return nil, nil, nil
}
func (m *mockPromAPI) LabelNames(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) ([]string, v1.Warnings, error) {
	return nil, nil, nil
}
func (m *mockPromAPI) LabelValues(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error) {
	return nil, nil, nil
}
func (m *mockPromAPI) Series(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) ([]model.LabelSet, v1.Warnings, error) {
	return nil, nil, nil
}
func (m *mockPromAPI) Alerts(ctx context.Context) (v1.AlertsResult, error) {
	return v1.AlertsResult{}, nil
}
func (m *mockPromAPI) AlertManagers(ctx context.Context) (v1.AlertManagersResult, error) {
	return v1.AlertManagersResult{}, nil
}
func (m *mockPromAPI) CleanTombstones(ctx context.Context) error { return nil }
func (m *mockPromAPI) Config(ctx context.Context) (v1.ConfigResult, error) {
	return v1.ConfigResult{}, nil
}
func (m *mockPromAPI) DeleteSeries(ctx context.Context, matches []string, startTime, endTime time.Time) error {
	return nil
}
func (m *mockPromAPI) Flags(ctx context.Context) (v1.FlagsResult, error) {
	return v1.FlagsResult{}, nil
}
func (m *mockPromAPI) Snapshot(ctx context.Context, skipHead bool) (v1.SnapshotResult, error) {
	return v1.SnapshotResult{}, nil
}
func (m *mockPromAPI) Rules(ctx context.Context) (v1.RulesResult, error) {
	return v1.RulesResult{}, nil
}
func (m *mockPromAPI) Targets(ctx context.Context) (v1.TargetsResult, error) {
	return v1.TargetsResult{}, nil
}
func (m *mockPromAPI) TargetsMetadata(ctx context.Context, matchTarget, metric, limit string) ([]v1.MetricMetadata, error) {
	return nil, nil
}
func (m *mockPromAPI) Metadata(ctx context.Context, metric, limit string) (map[string][]v1.Metadata, error) {
	return nil, nil
}
func (m *mockPromAPI) TSDB(ctx context.Context, opts ...v1.Option) (v1.TSDBResult, error) {
	return v1.TSDBResult{}, nil
}
func (m *mockPromAPI) WalReplay(ctx context.Context) (v1.WalReplayStatus, error) {
	return v1.WalReplayStatus{}, nil
}
func (m *mockPromAPI) Runtimeinfo(ctx context.Context) (v1.RuntimeinfoResult, error) {
	return v1.RuntimeinfoResult{}, nil
}
func (m *mockPromAPI) Buildinfo(ctx context.Context) (v1.BuildinfoResult, error) {
	return v1.BuildinfoResult{}, nil
}
func (m *mockPromAPI) QueryExemplars(ctx context.Context, query string, startTime, endTime time.Time) ([]v1.ExemplarQueryResult, error) {
	return nil, nil
}

var _ = Describe("Client", func() {

	// Test the setupPromClient function
	Describe("setupPromClient", func() {
		// Test successful client creation with valid parameters
		It("should create a Prometheus client with valid parameters", func() {
			// Valid Prometheus/Thanos URL and bearer token
			thanosURL := "thanos.example.com"
			bearerToken := "valid-token"
			insecureTLS := true

			// We setup the Prometheus client
			promClient, err := setupPromClient(thanosURL, bearerToken, insecureTLS)

			// The client should be created successfully
			Expect(err).NotTo(HaveOccurred())
			// The client should not be nil
			Expect(promClient).NotTo(BeNil())

		})

		// Test with empty bearer token (should still create client)
		It("should create client even with empty bearer token", func() {
			// Empty bearer token
			thanosURL := "thanos.example.com"
			bearerToken := ""

			// We create the client
			promClient, err := setupPromClient(thanosURL, bearerToken, true)

			// It should still succeed (auth might fail later, but client creation works)
			Expect(err).NotTo(HaveOccurred())
			Expect(promClient).NotTo(BeNil())
		})

		// Test with insecure TLS disabled
		It("should create client with insecure TLS disabled", func() {
			// insecureTLS set to false
			thanosURL := "thanos.example.com"
			bearerToken := "token"
			insecureTLS := false

			// We create the client
			promClient, err := setupPromClient(thanosURL, bearerToken, insecureTLS)

			// It should succeed
			Expect(err).NotTo(HaveOccurred())
			Expect(promClient).NotTo(BeNil())
		})
	})

	// Test executeQuery with mock Prometheus client
	Describe("executeQuery", func() {
		var (
			testDB      *sql.DB
			sqliteDB    database.Database
			clusterID   int64
			tmpDir      string
			originalDir string
		)

		// Setup: Create a test database before each test
		BeforeEach(func() {
			var err error
			// Save current directory
			originalDir, err = os.Getwd()
			Expect(err).NotTo(HaveOccurred())

			// Create a temporary directory for the test database
			tmpDir, err = os.MkdirTemp("", "prom-test-*")
			Expect(err).NotTo(HaveOccurred())

			// Change to temp directory so InitDB creates database there
			err = os.Chdir(tmpDir)
			Expect(err).NotTo(HaveOccurred())

			// Initialize the test database
			sqliteDB = database.NewSQLiteDB()
			testDB, err = sqliteDB.InitDB()
			Expect(err).NotTo(HaveOccurred())

			// Create a test cluster
			clusterID, err = sqliteDB.GetOrCreateCluster(testDB, "test-cluster", "")
			Expect(err).NotTo(HaveOccurred())
		})

		// Cleanup: Close database and remove temp files after each test
		AfterEach(func() {
			if testDB != nil {
				err := testDB.Close()
				Expect(err).NotTo(HaveOccurred())
			}
			// Change back to original directory
			if originalDir != "" {
				err := os.Chdir(originalDir)
				Expect(err).NotTo(HaveOccurred())
			}
			if tmpDir != "" {
				err := os.RemoveAll(tmpDir)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		// Test executeQuery with successful response
		It("should successfully execute query and store results with vector response", func() {
			// A mock client that returns a vector result (no actual server)
			mock := &mockPromAPI{
				queryFunc: func(ctx context.Context, query string, ts time.Time) (model.Value, v1.Warnings, error) {
					return model.Vector{
						&model.Sample{
							Metric:    model.Metric{"__name__": "up", "job": "prometheus"},
							Value:     1,
							Timestamp: model.Now(),
						},
					}, nil, nil
				},
			}

			// We execute a query with the mock client
			ctx := context.Background()
			info := output.QueryInfo{QueryID: "test-query-1", PromQuery: "up", Frequency: 5, SampleNumber: 1, TotalSamples: 4}
			executeQuery(ctx, mock, testDB, sqliteDB, clusterID, info)

			// Results should be stored in the database
			var count int
			err := testDB.QueryRow("SELECT COUNT(*) FROM query_results WHERE kpi_id = ?", "test-query-1").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(BeNumerically(">", 0))
		})

		// Test executeQuery with multiple metrics in response
		It("should successfully execute query and store results with multiple metrics", func() {
			// A mock client that returns multiple metrics (no actual server)
			mock := &mockPromAPI{
				queryFunc: func(ctx context.Context, query string, ts time.Time) (model.Value, v1.Warnings, error) {
					return model.Vector{
						&model.Sample{Metric: model.Metric{"__name__": "cpu_usage", "pod": "test-pod-1"}, Value: 0.5},
						&model.Sample{Metric: model.Metric{"__name__": "cpu_usage", "pod": "test-pod-2"}, Value: 0.7},
						&model.Sample{Metric: model.Metric{"__name__": "cpu_usage", "pod": "test-pod-3"}, Value: 0.9},
					}, nil, nil
				},
			}

			ctx := context.Background()
			info := output.QueryInfo{QueryID: "test-query-2", PromQuery: "cpu_usage", Frequency: 5, SampleNumber: 1, TotalSamples: 4}
			executeQuery(ctx, mock, testDB, sqliteDB, clusterID, info)

			// Multiple results should be stored
			var count int
			err := testDB.QueryRow("SELECT COUNT(*) FROM query_results WHERE kpi_id = ?", "test-query-2").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(3))
		})

		// Test executeQuery with query error
		It("should increment error count when query fails", func() {
			// A mock client that returns an error (no actual server)
			mock := &mockPromAPI{
				queryFunc: func(ctx context.Context, query string, ts time.Time) (model.Value, v1.Warnings, error) {
					return nil, nil, &v1.Error{Type: v1.ErrBadData, Msg: "invalid query syntax"}
				},
			}

			ctx := context.Background()
			queryID := "test-query-error"
			info := output.QueryInfo{QueryID: queryID, PromQuery: "invalid{query", Frequency: 5, SampleNumber: 1, TotalSamples: 4}
			executeQuery(ctx, mock, testDB, sqliteDB, clusterID, info)

			// Error count should be incremented in database
			var errorCount int
			err := testDB.QueryRow("SELECT errors FROM query_errors WHERE kpi_id = ?", queryID).Scan(&errorCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(errorCount).To(Equal(1))
		})

		// Test executeQuery with warnings
		It("should handle Prometheus warnings", func() {
			// A mock client that returns warnings (no actual server)
			mock := &mockPromAPI{
				queryFunc: func(ctx context.Context, query string, ts time.Time) (model.Value, v1.Warnings, error) {
					return model.Vector{&model.Sample{Metric: model.Metric{"__name__": "test"}, Value: 42}},
						v1.Warnings{"query took longer", "metrics dropped"},
						nil
				},
			}

			ctx := context.Background()
			info := output.QueryInfo{QueryID: "test-query-warnings", PromQuery: "test", Frequency: 5, SampleNumber: 1, TotalSamples: 4}
			executeQuery(ctx, mock, testDB, sqliteDB, clusterID, info)
		})

		// Test executeQuery with empty results
		It("should handle empty query results", func() {
			// A mock client that returns empty results (no actual server)
			mock := &mockPromAPI{
				queryFunc: func(ctx context.Context, query string, ts time.Time) (model.Value, v1.Warnings, error) {
					return model.Vector{}, nil, nil
				},
			}

			ctx := context.Background()
			info := output.QueryInfo{QueryID: "test-query-empty", PromQuery: "nonexistent_metric", Frequency: 5, SampleNumber: 1, TotalSamples: 4}
			executeQuery(ctx, mock, testDB, sqliteDB, clusterID, info)

			// No results should be stored
			var count int
			err := testDB.QueryRow("SELECT COUNT(*) FROM query_results WHERE kpi_id = ?", "test-query-empty").Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		// Test executeQuery with context timeout
		It("should respect context timeout", func() {
			// A mock client that simulates slow query (no actual server)
			mock := &mockPromAPI{
				queryFunc: func(ctx context.Context, query string, ts time.Time) (model.Value, v1.Warnings, error) {
					select {
					case <-ctx.Done():
						return nil, nil, ctx.Err()
					case <-time.After(2 * time.Second):
						return model.Vector{}, nil, nil
					}
				},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			info := output.QueryInfo{QueryID: "test-query-timeout", PromQuery: "slow_query", Frequency: 5, SampleNumber: 1, TotalSamples: 4}
			executeQuery(ctx, mock, testDB, sqliteDB, clusterID, info)
		})
	})

})
