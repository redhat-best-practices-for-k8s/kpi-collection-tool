package grafana_templates

import "embed"

//go:embed sqlite-dashboard.json postgres-dashboard.json provisioning-dashboards.yaml
var FS embed.FS

// File names for easy reference
const (
	SQLiteDashboard      = "sqlite-dashboard.json"
	PostgresDashboard    = "postgres-dashboard.json"
	ProvisioningDashboards = "provisioning-dashboards.yaml"
)

