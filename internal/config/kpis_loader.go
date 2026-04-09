package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/prometheus/prometheus/promql/parser"
)

// LoadKPIs loads Prometheus queries from kpis file
func LoadKPIs(filepath string) (KPIs, error) {
	kpisFile, err := os.Open(filepath)
	if err != nil {
		return KPIs{}, fmt.Errorf("failed to open kpis file: %v", err)
	}
	defer func() {
		if closeErr := kpisFile.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close kpis file: %v\n", closeErr)
		}
	}()

	var kpis KPIs
	decoder := json.NewDecoder(kpisFile)
	if err := decoder.Decode(&kpis); err != nil {
		return KPIs{}, fmt.Errorf("failed to decode kpis file: %v", err)
	}

	return kpis, nil
}

// ValidateKPIs checks all KPI queries for syntax errors and configuration issues.
// Returns a slice of errors found during validation.
func ValidateKPIs(kpis KPIs) []error {
	var errors []error
	seenIDs := make(map[string]bool)

	for _, kpi := range kpis.Queries {
		// Check for empty KPI ID
		if strings.TrimSpace(kpi.ID) == "" {
			errors = append(errors, fmt.Errorf("KPI has empty ID"))
			continue
		}

		// Check for duplicate IDs
		if seenIDs[kpi.ID] {
			errors = append(errors, fmt.Errorf("duplicate KPI ID: %s", kpi.ID))
		}
		seenIDs[kpi.ID] = true

		// Check for empty queries
		if strings.TrimSpace(kpi.PromQuery) == "" {
			errors = append(errors, fmt.Errorf("KPI '%s': empty PromQL query", kpi.ID))
			continue
		}

		// Validate PromQL syntax
		if _, err := parser.ParseExpr(kpi.PromQuery); err != nil {
			errors = append(errors, fmt.Errorf("KPI '%s': invalid PromQL syntax - %w", kpi.ID, err))
		}

		errors = append(errors, validateQueryType(kpi)...)
	}

	return errors
}

func validateQueryType(kpi Query) []error {
	var errors []error

	switch kpi.GetEffectiveQueryType() {
	case "instant":
		if kpi.Range != nil {
			errors = append(errors, fmt.Errorf("KPI '%s': range can only be set when query-type is 'range'", kpi.ID))
		}
	case "range":
		errors = append(errors, validateRangeWindow(kpi)...)
	default:
		errors = append(errors, fmt.Errorf("KPI '%s': invalid query-type '%s' (must be 'instant' or 'range')", kpi.ID, kpi.QueryType))
	}

	return errors
}

// validateRangeWindow checks that the range window is properly configured:
// step and since are required; until is optional (defaults to "now").
func validateRangeWindow(kpi Query) []error {
	var errors []error

	if kpi.Range == nil {
		errors = append(errors, fmt.Errorf("KPI '%s': range is required when query-type is 'range'", kpi.ID))
		return errors
	}

	rw := kpi.Range

	if rw.Step == nil || rw.Step.Duration <= 0 {
		errors = append(errors, fmt.Errorf("KPI '%s': range.step is required and must be > 0 when query-type is 'range'", kpi.ID))
	}

	if rw.Since == nil {
		errors = append(errors,
			fmt.Errorf("KPI '%s': range.since is required when query-type is 'range'", kpi.ID))
		return errors
	}

	errors = append(errors, validateTimestampPositive(kpi.ID, "since", rw.Since)...)
	if rw.Until != nil {
		errors = append(errors, validateTimestampPositive(kpi.ID, "until", rw.Until)...)
	}

	errors = append(errors, validateSinceBeforeUntil(kpi)...)

	return errors
}

func validateTimestampPositive(kpiID, field string, ts *Timestamp) []error {
	if ts.IsDuration() && ts.DurationValue() <= 0 {
		return []error{fmt.Errorf("KPI '%s': range.%s must be > 0 when specified as a duration", kpiID, field)}
	}
	return nil
}

func validateSinceBeforeUntil(kpi Query) []error {
	rw := kpi.Range
	if rw.Since == nil {
		return nil
	}

	if rw.Since.IsDuration() && rw.Until == nil {
		if rw.Step != nil && rw.Step.Duration > rw.Since.DurationValue() {
			return []error{fmt.Errorf("KPI '%s': step must be less than or equal to the since-until window", kpi.ID)}
		}
		return nil
	}

	if rw.Since.IsAbsolute() && rw.Until != nil && rw.Until.IsAbsolute() {
		start := rw.Since.AbsoluteValue()
		end := rw.Until.AbsoluteValue()
		if !start.Before(end) {
			return []error{fmt.Errorf("KPI '%s': since must be before until", kpi.ID)}
		}
		if rw.Step != nil && end.Sub(start) < rw.Step.Duration {
			return []error{fmt.Errorf("KPI '%s': step must be less than or equal to the since-until window", kpi.ID)}
		}
	}

	return nil
}
