package output

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/dustin/go-humanize"
)

func (p *Printer) printKPIsTable(records []KPIRecord) error {
	w := tabwriter.NewWriter(p.writer, 0, 0, 2, ' ', 0)

	if p.noTruncate {
		_, _ = fmt.Fprintln(w, "ID\tKPI_NAME\tCATEGORY\tCLUSTER\tVALUE\tTIMESTAMP\tEXECUTION_TIME")
		_, _ = fmt.Fprintln(w, "---\t---\t---\t---\t---\t---\t---")

		for _, r := range records {
			_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%.6f\t%.0f\t%s\n",
				r.ID, r.KPIName, categoryDisplay(r.Category), r.Cluster, r.Value,
				r.Timestamp, r.ExecutionTime.Format("2006-01-02 15:04:05"))
			_ = w.Flush()

			_, _ = fmt.Fprintln(p.writer, "  Labels:")
			p.printPrettyLabels(r.Labels)
			_, _ = fmt.Fprintln(p.writer)
		}
	} else {
		_, _ = fmt.Fprintln(w, "ID\tKPI_NAME\tCATEGORY\tCLUSTER\tVALUE\tTIMESTAMP\tEXECUTION_TIME\tLABELS")
		_, _ = fmt.Fprintln(w, "---\t---\t---\t---\t---\t---\t---\t---")

		for _, r := range records {
			labels := r.LabelsRaw
			if len(labels) > 50 {
				labels = labels[:47] + "..."
			}
			_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%.6f\t%.0f\t%s\t%s\n",
				r.ID, r.KPIName, categoryDisplay(r.Category), r.Cluster, r.Value,
				r.Timestamp, r.ExecutionTime.Format("2006-01-02 15:04:05"), labels)
		}
		_ = w.Flush()
	}

	_, _ = fmt.Fprintf(p.writer, "\nTotal results: %d\n", len(records))
	return nil
}

func categoryDisplay(category string) string {
	if category == "" {
		return "-"
	}
	return category
}

func (p *Printer) printPrettyLabels(labels map[string]string) {
	if labels == nil {
		return
	}
	for key, value := range labels {
		_, _ = fmt.Fprintf(p.writer, "    %s: %s\n", key, value)
	}
}

// PrintClustersTable prints cluster records as a table to stdout
func PrintClustersTable(records []ClusterRecord) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tCLUSTER_NAME\tCREATED_AT\tTOTAL_METRICS")
	_, _ = fmt.Fprintln(w, "---\t---\t---\t---")

	for _, c := range records {
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\n",
			c.ID, c.Name, c.CreatedAt.Format("2006-01-02 15:04:05"),
			humanize.Comma(c.TotalMetrics))
	}
	_ = w.Flush()
}

// PrintErrorsTable prints error records as a table to stdout
func PrintErrorsTable(records []ErrorRecord) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "KPI_ID\tERROR_COUNT")
	_, _ = fmt.Fprintln(w, "---\t---")

	for _, e := range records {
		_, _ = fmt.Fprintf(w, "%s\t%d\n", e.KPIID, e.ErrorCount)
	}
	_ = w.Flush()
}
