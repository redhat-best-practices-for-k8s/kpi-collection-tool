package taskprofiles

import "embed"

//go:embed per-node-data-full.yaml oslat-full.yaml app-recovery-time-full.yaml
var FS embed.FS

const (
	PerNodeDataFull     = "per-node-data-full.yaml"
	OslatFull           = "oslat-full.yaml"
	AppRecoveryTimeFull = "app-recovery-time-full.yaml"
)
