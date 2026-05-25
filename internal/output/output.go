// Package output provides multi-format output rendering for CLI commands.
// Supports table (human-readable), JSON, and CSV formats.
package output

import (
	"fmt"
	"io"
	"os"
	"time"
)

// Format represents the output format type
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatCSV   Format = "csv"
)

// ParseFormat converts a string to a Format, returning an error if invalid
func ParseFormat(s string) (Format, error) {
	switch s {
	case "table", "":
		return FormatTable, nil
	case "json":
		return FormatJSON, nil
	case "csv":
		return FormatCSV, nil
	default:
		return "", fmt.Errorf("invalid output format %q: must be table, json, or csv", s)
	}
}

// KPIRecord represents a single KPI metric for output
type KPIRecord struct {
	ID            int64             `json:"id"`
	KPIName       string            `json:"kpi_name"`
	Category      string            `json:"category,omitempty"`
	Cluster       string            `json:"cluster"`
	Value         float64           `json:"value"`
	Timestamp     float64           `json:"timestamp"`
	ExecutionTime time.Time         `json:"execution_time"`
	Labels        map[string]string `json:"labels"`
	LabelsRaw     string            `json:"-"` // Original JSON string, used for table display
}

// ClusterRecord represents a cluster info record for output
type ClusterRecord struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"created_at"`
	TotalMetrics int64     `json:"total_metrics"`
}

// ErrorRecord represents a query error record for output
type ErrorRecord struct {
	KPIID      string `json:"kpi_id"`
	ErrorCount int    `json:"error_count"`
}

// Printer handles output formatting
type Printer struct {
	format     Format
	writer     io.Writer
	noTruncate bool // For table format: show full labels
}

// NewPrinter creates a new Printer with the specified format
func NewPrinter(format Format) *Printer {
	return &Printer{
		format: format,
		writer: os.Stdout,
	}
}

// WithWriter sets a custom writer (useful for testing)
func (p *Printer) WithWriter(w io.Writer) *Printer {
	p.writer = w
	return p
}

// WithNoTruncate disables label truncation for table format
func (p *Printer) WithNoTruncate(noTruncate bool) *Printer {
	p.noTruncate = noTruncate
	return p
}

// PrintKPIs outputs KPI records in the configured format
func (p *Printer) PrintKPIs(records []KPIRecord) error {
	switch p.format {
	case FormatJSON:
		return p.printKPIsJSON(records)
	case FormatCSV:
		return p.printKPIsCSV(records)
	default:
		return p.printKPIsTable(records)
	}
}

