package output

import (
	"encoding/json"
)

func (p *Printer) printKPIsJSON(records []KPIRecord) error {
	encoder := json.NewEncoder(p.writer)
	encoder.SetIndent("", "  ")

	if p.showExecTime {
		return encoder.Encode(records)
	}

	type kpiRecordNoExec struct {
		ID        int64             `json:"id"`
		KPIName   string            `json:"kpi_name"`
		Cluster   string            `json:"cluster"`
		Value     float64           `json:"value"`
		Timestamp string            `json:"timestamp"`
		Labels    map[string]string `json:"labels"`
	}

	slim := make([]kpiRecordNoExec, len(records))
	for i, r := range records {
		slim[i] = kpiRecordNoExec{
			ID:        r.ID,
			KPIName:   r.KPIName,
			Cluster:   r.Cluster,
			Value:     r.Value,
			Timestamp: r.Timestamp,
			Labels:    r.Labels,
		}
	}
	return encoder.Encode(slim)
}
