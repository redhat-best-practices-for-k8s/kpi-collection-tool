package kpiprofiles

import "embed"

//go:embed kpis-ran.json kpis-core.json kpis-hub.json
var FS embed.FS

const (
	RAN  = "kpis-ran.json"
	Core = "kpis-core.json"
	Hub  = "kpis-hub.json"
)
