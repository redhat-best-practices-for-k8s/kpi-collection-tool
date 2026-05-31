package output

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/dustin/go-humanize"
)

func (p *Printer) printKPIsTable(records []KPIRecord) error {
	w := tabwriter.NewWriter(p.writer, 0, 0, 2, ' ', 0)

	if p.noTruncate {
		header, separator := p.buildKPITableHeader(false)
		_, _ = fmt.Fprintln(w, header)
		_, _ = fmt.Fprintln(w, separator)

		for _, r := range records {
			_, _ = fmt.Fprint(w, p.buildKPITableRow(r, ""))
			_ = w.Flush()

			_, _ = fmt.Fprintln(p.writer, "  Labels:")
			p.printPrettyLabels(r.Labels)
			_, _ = fmt.Fprintln(p.writer)
		}
	} else {
		header, separator := p.buildKPITableHeader(true)
		_, _ = fmt.Fprintln(w, header)
		_, _ = fmt.Fprintln(w, separator)

		for _, r := range records {
			labels := r.LabelsRaw
			if len(labels) > 50 {
				labels = labels[:47] + "..."
			}
			_, _ = fmt.Fprint(w, p.buildKPITableRow(r, labels))
		}
		_ = w.Flush()
	}

	_, _ = fmt.Fprintf(p.writer, "\nTotal results: %d\n", len(records))
	return nil
}

func (p *Printer) buildKPITableHeader(includeLabels bool) (header, separator string) {
	header = "ID\tKPI_NAME\tCATEGORY\tCLUSTER\tVALUE\tTIMESTAMP"
	separator = "---\t---\t---\t---\t---\t---"

	if p.showExecTime {
		header += "\tEXECUTION_TIME"
		separator += "\t---"
	}
	if includeLabels {
		header += "\tLABELS"
		separator += "\t---"
	}
	return header, separator
}

func (p *Printer) buildKPITableRow(r KPIRecord, labels string) string {
	value := strconv.FormatFloat(r.Value, 'f', -1, 64)
	row := fmt.Sprintf("%d\t%s\t%s\t%s\t%s\t%s",
		r.ID, r.KPIName, categoryDisplay(r.Category), r.Cluster, value, r.Timestamp)

	if p.showExecTime {
		row += "\t" + r.ExecutionTime.Format("2006-01-02 15:04:05")
	}
	if labels != "" {
		row += "\t" + labels
	}
	return row + "\n"
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

// PrintCategoriesTable prints category records as a table to stdout
func PrintCategoriesTable(records []CategoryRecord) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "CATEGORY\tTABLE\tKPIS")
	_, _ = fmt.Fprintln(w, "---\t---\t---")

	for _, c := range records {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\n", c.Category, c.TableName, c.KPICount)
	}
	_ = w.Flush()
}
