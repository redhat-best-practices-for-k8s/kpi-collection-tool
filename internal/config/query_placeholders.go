package config

import (
	"strings"
)

// CPUPlaceholders holds CPU IDs in Prometheus regex format for query substitution
type CPUPlaceholders struct {
	Reserved string // e.g., "0|1|32|33"
	Isolated string // e.g., "2|3|34|35"
}

// RequiresCPUSubstitution checks if any KPI query contains CPU placeholders
func RequiresCPUSubstitution(kpis KPIs) bool {
	for _, kpi := range kpis.Queries {
		if strings.Contains(kpi.PromQuery, "{{RESERVED_CPUS}}") ||
			strings.Contains(kpi.PromQuery, "{{ISOLATED_CPUS}}") {
			return true
		}
	}
	return false
}

// SubstituteCPUPlaceholders replaces CPU placeholders in all KPI queries
func SubstituteCPUPlaceholders(kpis KPIs, cpus *CPUPlaceholders) KPIs {
	if cpus == nil {
		return kpis
	}

	resolvedKPIs := KPIs{
		Queries: make([]Query, len(kpis.Queries)),
	}

	for i, originalQuery := range kpis.Queries {
		resolvedPromQL := originalQuery.PromQuery
		resolvedPromQL = strings.ReplaceAll(resolvedPromQL, "{{RESERVED_CPUS}}", cpus.Reserved)
		resolvedPromQL = strings.ReplaceAll(resolvedPromQL, "{{ISOLATED_CPUS}}", cpus.Isolated)

		resolvedKPIs.Queries[i] = originalQuery
		resolvedKPIs.Queries[i].PromQuery = resolvedPromQL
	}

	return resolvedKPIs
}
