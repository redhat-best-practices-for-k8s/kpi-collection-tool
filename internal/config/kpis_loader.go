package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

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
	now := time.Now()
	rw := kpi.Range

	if rw == nil {
		return []error{fmt.Errorf("KPI '%s': range is required when query-type is 'range'", kpi.ID)}
	}

	var errors []error
	if rw.Step == nil {
		errors = append(errors, fmt.Errorf("KPI '%s': range.step is required when query-type is 'range'", kpi.ID))
	} else if rw.Step.Duration <= 0 {
		errors = append(errors, fmt.Errorf("KPI '%s': range.step must be > 0 when query-type is 'range'", kpi.ID))
	}

	if rw.Since == nil {
		errors = append(errors, fmt.Errorf("KPI '%s': range.since is required when query-type is 'range'", kpi.ID))
	} else if err := validateTimeRefPositive(kpi.ID, "since", rw.Since); err != nil {
		errors = append(errors, err)
	}

	if rw.Until != nil {
		if err := validateTimeRefPositive(kpi.ID, "until", rw.Until); err != nil {
			errors = append(errors, err)
		}
	}

	// No need to check since before until if there are errors or until is not set
	if len(errors) > 0 {
		return errors
	}

	since := rw.Since.Resolve(now)
	if rw.Until == nil {
		rangeDuration := now.Sub(since)
		if rangeDuration < rw.Step.Duration {
			fmt.Printf("WARNING: KPI '%s': step is greater than range window duration (step: %s, range duration: %s)\n",
				kpi.ID, rw.Step.String(), rangeDuration.String())
		}

		return nil
	}

	until := rw.Until.Resolve(now)
	if !since.Before(until) {
		return []error{fmt.Errorf("KPI '%s': since must be before until (since: %s, until: %s)",
			kpi.ID, since, until)}
	}

	rangeDuration := until.Sub(since)
	if rangeDuration < rw.Step.Duration {
		fmt.Printf("WARNING: KPI '%s': step is greater than range window duration (step: %s, range duration: %s)\n",
			kpi.ID, rw.Step.String(), rangeDuration.String())
	}

	return nil
}

func validateTimeRefPositive(kpiID, field string, timeRef *TimeRef) error {
	if timeRef.IsDuration() && timeRef.DurationValue() <= 0 {
		return fmt.Errorf("KPI '%s': range.%s must be > 0 when specified as a duration", kpiID, field)
	}

	return nil
}
