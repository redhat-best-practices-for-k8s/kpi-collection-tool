package output

import (
	"encoding/csv"
	"encoding/json"
	"strconv"
)

func (p *Printer) printKPIsCSV(records []KPIRecord) error {
	w := csv.NewWriter(p.writer)
	defer w.Flush()

	if err := w.Write([]string{"id", "kpi_name", "category", "cluster", "value", "timestamp", "execution_time", "labels"}); err != nil {
		return err
	}

	for _, r := range records {
		labelsJSON, _ := json.Marshal(r.Labels)
		row := []string{
			strconv.FormatInt(r.ID, 10),
			r.KPIName,
			r.Category,
			r.Cluster,
			strconv.FormatFloat(r.Value, 'f', 6, 64),
			strconv.FormatFloat(r.Timestamp, 'f', 0, 64),
			r.ExecutionTime.Format("2006-01-02 15:04:05"),
			string(labelsJSON),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return w.Error()
}

