// Package config provides configuration types, validation, and loading
// for the KPI collector. It defines input flags, KPI query structures,
// and validation logic for command-line arguments.
package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Duration is a wrapper around time.Duration that supports JSON unmarshaling
// from both duration strings (e.g., "30s", "2m", "1h") and integer seconds
type Duration struct {
	time.Duration
}

// UnmarshalJSON implements json.Unmarshaler for Duration
// Supports both string format ("30s", "2m", "1h") and integer seconds (60)
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	switch value := v.(type) {
	case float64:
		// JSON numbers are float64, treat as seconds for backward compatibility
		d.Duration = time.Duration(value) * time.Second
		return nil
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid duration string %q: %w", value, err)
		}
		return nil
	default:
		return fmt.Errorf("invalid duration type: %T", v)
	}
}

// MarshalJSON implements json.Marshaler for Duration
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// TimeRef represents a flexible time specification that can be either a Go
// duration string (e.g. "2h", "1m30s") interpreted as relative to "now", or an
// absolute RFC 3339 time reference (e.g. "2026-04-07T12:24:25Z").
type TimeRef struct {
	duration *time.Duration
	absolute *time.Time
}

// IsDuration returns true when the time reference holds a relative duration.
func (t *TimeRef) IsDuration() bool { return t.duration != nil }

// IsAbsolute returns true when the time reference holds an absolute point in time.
func (t *TimeRef) IsAbsolute() bool { return t.absolute != nil }

// DurationValue returns the contained duration. Panics if not a duration.
func (t *TimeRef) DurationValue() time.Duration { return *t.duration }

// AbsoluteValue returns the contained time. Panics if not absolute.
func (t *TimeRef) AbsoluteValue() time.Time { return *t.absolute }

// Resolve converts the TimeRef to an absolute time.Time.
// Durations are subtracted from the supplied reference time (typically time.Now()).
func (t *TimeRef) Resolve(now time.Time) time.Time {
	if t.absolute != nil {
		return *t.absolute
	}
	return now.Add(-*t.duration)
}

// UnmarshalJSON implements json.Unmarshaler for TimeRef.
// Accepts either a Go duration string ("2h", "30m") or an RFC 3339 time reference.
func (t *TimeRef) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("time reference must be a string: %w", err)
	}

	s = strings.TrimSpace(s)

	if d, err := time.ParseDuration(s); err == nil {
		t.duration = &d
		return nil
	}

	if parsed, err := time.Parse(time.RFC3339, s); err == nil {
		t.absolute = &parsed
		return nil
	}

	return fmt.Errorf("invalid time reference %q: must be a Go duration (e.g. \"2h\") or RFC 3339 format (e.g. \"2026-04-07T12:24:25Z\")", s)
}

// MarshalJSON implements json.Marshaler for TimeRef
func (t TimeRef) MarshalJSON() ([]byte, error) {
	if t.duration != nil {
		return json.Marshal(t.duration.String())
	}
	if t.absolute != nil {
		return json.Marshal(t.absolute.Format(time.RFC3339))
	}
	return json.Marshal(nil)
}

// RangeWindow defines the time window for a range query.
// Step is always required. Since is required; Until is optional (defaults to "now").
// Both Since and Until accept either a Go duration ("2h") or an RFC 3339 timestamp.
type RangeWindow struct {
	Step  *Duration `json:"step"`
	Since *TimeRef  `json:"since"`
	Until *TimeRef  `json:"until,omitempty"`
}

// InputFlags holds all command line flag values
type InputFlags struct {
	BearerToken  string
	ThanosURL    string
	Kubeconfig   string
	ClusterName  string
	ClusterType  string
	InsecureTLS  bool
	SamplingFreq time.Duration
	Duration     time.Duration
	DatabaseType string // "sqlite" or "postgres"
	PostgresURL  string // PostgreSQL connection string
	KPIsFile     string
	SingleRun    bool // collect metrics once and exit
}

// Query represents a single KPI query configuration
type Query struct {
	ID              string       `json:"id"`
	PromQuery       string       `json:"promquery"`
	SampleFrequency *Duration    `json:"sample-frequency,omitempty"`
	QueryType       string       `json:"query-type,omitempty"`
	Range           *RangeWindow `json:"range,omitempty"`
	RunOnce         *bool        `json:"run-once,omitempty"`
}

// IsRunOnce returns true if this query is configured to run only once
func (q *Query) IsRunOnce() bool {
	return q.RunOnce != nil && *q.RunOnce
}

// KPIs represents the structure of the kpis.json file containing
// the list of KPI queries to be executed against Prometheus/Thanos
type KPIs struct {
	Queries []Query `json:"kpis"`
}

// GetEffectiveFrequency returns the sample frequency for this query,
// falling back to the provided default if not specified
func (q *Query) GetEffectiveFrequency(defaultFreq time.Duration) time.Duration {
	if q.SampleFrequency != nil && q.SampleFrequency.Duration > 0 {
		return q.SampleFrequency.Duration
	}
	return defaultFreq
}

// GetEffectiveQueryType returns the query type for this query,
// defaulting to "instant" if not specified
func (q *Query) GetEffectiveQueryType() string {
	if qt := strings.TrimSpace(q.QueryType); qt != "" {
		return qt
	}
	return "instant"
}
