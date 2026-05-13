package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/prometheus/promql/parser"
	"gopkg.in/yaml.v3"
)

const maxCategoryLength = 32

var categoryCleanRE = regexp.MustCompile(`[^a-z0-9_]`)

// LoadKPIs loads Prometheus queries from a YAML kpis file
func LoadKPIs(filepath string) (KPIs, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return KPIs{}, fmt.Errorf("failed to open kpis file: %v", err)
	}

	var kpis KPIs
	if err := yaml.Unmarshal(data, &kpis); err != nil {
		return KPIs{}, fmt.Errorf("failed to decode kpis file: %v", err)
	}

	return kpis, nil
}

// ValidateKPIs checks all KPI queries for syntax errors and configuration issues.
// Returns a slice of errors found during validation.
func ValidateKPIs(kpis KPIs) []error {
	var errors []error
	seenIDs := make(map[string]bool)

	for i := range kpis.Queries {
		kpi := &kpis.Queries[i]

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

		errors = append(errors, validateCategory(kpi)...)
		errors = append(errors, validateQueryType(*kpi)...)
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

// SanitizeCategory normalises a user-supplied category string into a safe,
// lowercase, underscore-separated identifier suitable for use as a SQL table
// name suffix. Returns an error if the sanitised result exceeds 32 characters
// or is empty.
func SanitizeCategory(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	s = categoryCleanRE.ReplaceAllString(s, "")
	s = strings.Trim(s, "_")

	if s == "" {
		return "", fmt.Errorf("category %q is empty after sanitisation", raw)
	}

	if len(s) > maxCategoryLength {
		return "", fmt.Errorf("category %q exceeds %d characters after sanitisation (%d chars: %q)",
			raw, maxCategoryLength, len(s), s)
	}

	return s, nil
}

func validateCategory(kpi *Query) []error {
	if kpi.Category == "" {
		return nil
	}

	var err error
	if kpi.Category, err = SanitizeCategory(kpi.Category); err != nil {
		return []error{fmt.Errorf("KPI '%s': %w", kpi.ID, err)}
	}

	return nil
}
