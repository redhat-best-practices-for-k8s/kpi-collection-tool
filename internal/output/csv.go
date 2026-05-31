package output

import (
	"encoding/csv"
	"encoding/json"
	"strconv"
)

func (p *Printer) printKPIsCSV(records []KPIRecord) error {
	w := csv.NewWriter(p.writer)
	defer w.Flush()

	header := []string{"id", "kpi_name", "category", "cluster", "value", "timestamp"}
	if p.showExecTime {
		header = append(header, "execution_time")
	}
	header = append(header, "labels")

	if err := w.Write(header); err != nil {
		return err
	}

	for _, r := range records {
		labelsJSON, _ := json.Marshal(r.Labels)
		row := []string{
			strconv.FormatInt(r.ID, 10),
			r.KPIName,
			r.Category,
			r.Cluster,
			strconv.FormatFloat(r.Value, 'f', -1, 64),
			r.Timestamp,
		}
		if p.showExecTime {
			row = append(row, r.ExecutionTime.Format("2006-01-02 15:04:05"))
		}
		row = append(row, string(labelsJSON))

		if err := w.Write(row); err != nil {
			return err
		}
	}

	return w.Error()
}

