package config

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("KPIs Loader", func() {
	var (
		tmpDir string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "kpis-loader-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if tmpDir != "" {
			err := os.RemoveAll(tmpDir)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Describe("LoadKPIs", func() {
		Context("when loading a valid KPIs file", func() {
			It("should load KPIs without custom frequency", func() {
				kpisJSON := `{
					"kpis": [
						{
							"id": "cpu-usage",
							"promquery": "avg(rate(node_cpu_seconds_total[5m]))"
						}
					]
				}`
				kpisPath := filepath.Join(tmpDir, "kpis.json")
				err := os.WriteFile(kpisPath, []byte(kpisJSON), 0644)
				Expect(err).NotTo(HaveOccurred())

				kpis, err := LoadKPIs(kpisPath)

				Expect(err).NotTo(HaveOccurred())
				Expect(kpis.Queries).To(HaveLen(1))
				Expect(kpis.Queries[0].ID).To(Equal("cpu-usage"))
				Expect(kpis.Queries[0].PromQuery).To(Equal("avg(rate(node_cpu_seconds_total[5m]))"))
				Expect(kpis.Queries[0].SampleFrequency).To(BeNil())
			})

			It("should load KPIs with custom frequency as integer seconds", func() {
				kpisJSON := `{
					"kpis": [
						{
							"id": "memory-usage",
							"promquery": "node_memory_MemTotal_bytes",
							"sample-frequency": 120
						}
					]
				}`
				kpisPath := filepath.Join(tmpDir, "kpis.json")
				err := os.WriteFile(kpisPath, []byte(kpisJSON), 0644)
				Expect(err).NotTo(HaveOccurred())

				kpis, err := LoadKPIs(kpisPath)

				Expect(err).NotTo(HaveOccurred())
				Expect(kpis.Queries).To(HaveLen(1))
				Expect(kpis.Queries[0].ID).To(Equal("memory-usage"))
				Expect(kpis.Queries[0].SampleFrequency).NotTo(BeNil())
				Expect(kpis.Queries[0].SampleFrequency.Duration).To(Equal(120 * time.Second))
			})

			It("should load KPIs with custom frequency as duration string", func() {
				kpisJSON := `{
					"kpis": [
						{
							"id": "disk-usage",
							"promquery": "node_filesystem_avail_bytes",
							"sample-frequency": "2m30s"
						}
					]
				}`
				kpisPath := filepath.Join(tmpDir, "kpis.json")
				err := os.WriteFile(kpisPath, []byte(kpisJSON), 0644)
				Expect(err).NotTo(HaveOccurred())

				kpis, err := LoadKPIs(kpisPath)

				Expect(err).NotTo(HaveOccurred())
				Expect(kpis.Queries).To(HaveLen(1))
				Expect(kpis.Queries[0].SampleFrequency).NotTo(BeNil())
				Expect(kpis.Queries[0].SampleFrequency.Duration).To(Equal(2*time.Minute + 30*time.Second))
			})

			It("should load multiple KPIs", func() {
				kpisJSON := `{
					"kpis": [
						{"id": "kpi-1", "promquery": "query1"},
						{"id": "kpi-2", "promquery": "query2"},
						{"id": "kpi-3", "promquery": "query3", "sample-frequency": 60}
					]
				}`
				kpisPath := filepath.Join(tmpDir, "kpis.json")
				err := os.WriteFile(kpisPath, []byte(kpisJSON), 0644)
				Expect(err).NotTo(HaveOccurred())

				kpis, err := LoadKPIs(kpisPath)

				Expect(err).NotTo(HaveOccurred())
				Expect(kpis.Queries).To(HaveLen(3))
				Expect(kpis.Queries[0].ID).To(Equal("kpi-1"))
				Expect(kpis.Queries[1].ID).To(Equal("kpi-2"))
				Expect(kpis.Queries[2].ID).To(Equal("kpi-3"))
			})

			It("should load an empty KPIs array", func() {
				kpisJSON := `{"kpis": []}`
				kpisPath := filepath.Join(tmpDir, "kpis.json")
				err := os.WriteFile(kpisPath, []byte(kpisJSON), 0644)
				Expect(err).NotTo(HaveOccurred())

				kpis, err := LoadKPIs(kpisPath)

				Expect(err).NotTo(HaveOccurred())
				Expect(kpis.Queries).To(BeEmpty())
			})
		})

		Context("when the file does not exist", func() {
			It("should return an error", func() {
				nonExistentPath := filepath.Join(tmpDir, "nonexistent.json")

				kpis, err := LoadKPIs(nonExistentPath)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to open kpis file"))
				Expect(kpis.Queries).To(BeNil())
			})
		})

		Context("when the file contains invalid JSON", func() {
			It("should return an error for malformed JSON", func() {
				invalidJSON := `{"kpis": [{"id": "test"`
				kpisPath := filepath.Join(tmpDir, "invalid.json")
				err := os.WriteFile(kpisPath, []byte(invalidJSON), 0644)
				Expect(err).NotTo(HaveOccurred())

				kpis, err := LoadKPIs(kpisPath)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to decode kpis file"))
				Expect(kpis.Queries).To(BeNil())
			})

			It("should return an error for invalid duration string", func() {
				invalidDuration := `{
					"kpis": [
						{
							"id": "test",
							"promquery": "test",
							"sample-frequency": "invalid-duration"
						}
					]
				}`
				kpisPath := filepath.Join(tmpDir, "invalid-duration.json")
				err := os.WriteFile(kpisPath, []byte(invalidDuration), 0644)
				Expect(err).NotTo(HaveOccurred())

				kpis, err := LoadKPIs(kpisPath)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to decode kpis file"))
				Expect(kpis.Queries).To(BeNil())
			})
		})

		Context("when the file is empty", func() {
			It("should return an error", func() {
				emptyPath := filepath.Join(tmpDir, "empty.json")
				err := os.WriteFile(emptyPath, []byte(""), 0644)
				Expect(err).NotTo(HaveOccurred())

				kpis, err := LoadKPIs(emptyPath)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to decode kpis file"))
				Expect(kpis.Queries).To(BeNil())
			})
		})
	})

	Describe("ValidateKPIs", func() {
		Context("when all KPIs are valid", func() {
			It("should return no errors for valid PromQL queries", func() {
				kpis := KPIs{
					Queries: []Query{
						{ID: "cpu-usage", PromQuery: "avg(rate(node_cpu_seconds_total[5m]))"},
						{ID: "memory-usage", PromQuery: "node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes"},
					},
				}

				errors := ValidateKPIs(kpis)

				Expect(errors).To(BeEmpty())
			})

			It("should return no errors for empty KPIs list", func() {
				kpis := KPIs{Queries: []Query{}}

				errors := ValidateKPIs(kpis)

				Expect(errors).To(BeEmpty())
			})
		})

		Context("when KPIs have invalid PromQL syntax", func() {
			It("should return an error for invalid PromQL", func() {
				kpis := KPIs{
					Queries: []Query{
						{ID: "broken-query", PromQuery: "rate(metric[5m]"},
					},
				}

				errors := ValidateKPIs(kpis)

				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("broken-query"))
				Expect(errors[0].Error()).To(ContainSubstring("invalid PromQL syntax"))
			})

			It("should return multiple errors for multiple invalid queries", func() {
				kpis := KPIs{
					Queries: []Query{
						{ID: "broken-1", PromQuery: "rate(metric[5m]"},
						{ID: "valid", PromQuery: "up"},
						{ID: "broken-2", PromQuery: "sum("},
					},
				}

				errors := ValidateKPIs(kpis)

				Expect(errors).To(HaveLen(2))
			})
		})

		Context("when KPIs have duplicate IDs", func() {
			It("should return an error for duplicate IDs", func() {
				kpis := KPIs{
					Queries: []Query{
						{ID: "cpu-usage", PromQuery: "up"},
						{ID: "cpu-usage", PromQuery: "down"},
					},
				}

				errors := ValidateKPIs(kpis)

				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("duplicate KPI ID"))
				Expect(errors[0].Error()).To(ContainSubstring("cpu-usage"))
			})

			It("should detect multiple duplicates", func() {
				kpis := KPIs{
					Queries: []Query{
						{ID: "kpi-a", PromQuery: "up"},
						{ID: "kpi-b", PromQuery: "up"},
						{ID: "kpi-a", PromQuery: "up"},
						{ID: "kpi-b", PromQuery: "up"},
					},
				}

				errors := ValidateKPIs(kpis)

				Expect(errors).To(HaveLen(2))
			})
		})

		Context("when KPIs have empty fields", func() {
			It("should return an error for empty KPI ID", func() {
				kpis := KPIs{
					Queries: []Query{
						{ID: "", PromQuery: "up"},
					},
				}

				errors := ValidateKPIs(kpis)

				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("empty ID"))
			})

			It("should return an error for whitespace-only KPI ID", func() {
				kpis := KPIs{
					Queries: []Query{
						{ID: "   ", PromQuery: "up"},
					},
				}

				errors := ValidateKPIs(kpis)

				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("empty ID"))
			})

			It("should return an error for empty PromQL query", func() {
				kpis := KPIs{
					Queries: []Query{
						{ID: "empty-query", PromQuery: ""},
					},
				}

				errors := ValidateKPIs(kpis)

				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("empty-query"))
				Expect(errors[0].Error()).To(ContainSubstring("empty PromQL query"))
			})

			It("should return an error for whitespace-only PromQL query", func() {
				kpis := KPIs{
					Queries: []Query{
						{ID: "whitespace-query", PromQuery: "   "},
					},
				}

				errors := ValidateKPIs(kpis)

				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("empty PromQL query"))
			})
		})

		Context("when KPIs have multiple validation issues", func() {
			It("should return all errors", func() {
				kpis := KPIs{
					Queries: []Query{
						{ID: "valid", PromQuery: "up"},
						{ID: "broken", PromQuery: "rate(metric[5m]"},
						{ID: "valid", PromQuery: "up"},
						{ID: "empty-query", PromQuery: ""},
					},
				}

				errors := ValidateKPIs(kpis)

				Expect(errors).To(HaveLen(3))
			})
		})

		Context("when validating range query configuration", func() {
			It("should allow valid range query configuration with since as duration", func() {
				sinceVal := time.Hour
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "cpu-range",
							PromQuery: "rate(node_cpu_seconds_total[5m])",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 30 * time.Second},
								Since: &TimeRef{duration: &sinceVal},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(BeEmpty())
			})

			It("should allow valid since+until both as durations", func() {
				sinceVal := 2 * time.Hour
				untilVal := time.Hour
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "cpu-range-until",
							PromQuery: "rate(node_cpu_seconds_total[5m])",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 30 * time.Second},
								Since: &TimeRef{duration: &sinceVal},
								Until: &TimeRef{duration: &untilVal},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(BeEmpty())
			})

			It("should reject since after until when both are durations", func() {
				sinceVal := 2 * time.Hour
				untilVal := 4 * time.Hour
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "bad-dur-order",
							PromQuery: "rate(node_cpu_seconds_total[5m])",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 30 * time.Second},
								Since: &TimeRef{duration: &sinceVal},
								Until: &TimeRef{duration: &untilVal},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("since must be before until"))
			})

			It("should reject since equal to until when both are durations", func() {
				d := 2 * time.Hour
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "equal-dur",
							PromQuery: "rate(node_cpu_seconds_total[5m])",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 30 * time.Second},
								Since: &TimeRef{duration: &d},
								Until: &TimeRef{duration: &d},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("since must be before until"))
			})

			It("should warn but not error when step is larger than since-until window (both durations)", func() {
				sinceVal := 2 * time.Hour
				untilVal := time.Hour
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "big-step-dur",
							PromQuery: "rate(node_cpu_seconds_total[5m])",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 2 * time.Hour},
								Since: &TimeRef{duration: &sinceVal},
								Until: &TimeRef{duration: &untilVal},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(BeEmpty())
			})

			It("should reject step equal to zero", func() {
				sinceVal := time.Hour
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "zero-step",
							PromQuery: "up",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 0},
								Since: &TimeRef{duration: &sinceVal},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("range.step must be > 0"))
			})

			It("should reject negative step", func() {
				sinceVal := time.Hour
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "negative-step",
							PromQuery: "up",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: -30 * time.Second},
								Since: &TimeRef{duration: &sinceVal},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("range.step must be > 0"))
			})

			It("should reject since duration equal to zero", func() {
				sinceVal := time.Duration(0)
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "zero-since",
							PromQuery: "up",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 30 * time.Second},
								Since: &TimeRef{duration: &sinceVal},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("range.since must be > 0"))
			})

			It("should reject negative since duration", func() {
				sinceVal := -time.Hour
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "negative-since",
							PromQuery: "up",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 30 * time.Second},
								Since: &TimeRef{duration: &sinceVal},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("range.since must be > 0"))
			})

			It("should reject until duration equal to zero", func() {
				sinceVal := time.Hour
				untilVal := time.Duration(0)
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "zero-until",
							PromQuery: "up",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 30 * time.Second},
								Since: &TimeRef{duration: &sinceVal},
								Until: &TimeRef{duration: &untilVal},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("range.until must be > 0"))
			})

			It("should reject negative until duration", func() {
				sinceVal := 2 * time.Hour
				untilVal := -time.Hour
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "negative-until",
							PromQuery: "up",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 30 * time.Second},
								Since: &TimeRef{duration: &sinceVal},
								Until: &TimeRef{duration: &untilVal},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("range.until must be > 0"))
			})

			It("should reject invalid query-type value", func() {
				kpis := KPIs{
					Queries: []Query{
						{ID: "bad-type", PromQuery: "up", QueryType: "window"},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("invalid query-type"))
			})

			It("should require range for range query type", func() {
				kpis := KPIs{
					Queries: []Query{
						{ID: "missing-range-fields", PromQuery: "up", QueryType: "range"},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("range is required"))
			})

			It("should require step inside range", func() {
				sinceVal := time.Hour
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "missing-step",
							PromQuery: "up",
							QueryType: "range",
							Range:     &RangeWindow{Since: &TimeRef{duration: &sinceVal}},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("step is required"))
			})

			It("should warn but not error when step is larger than since-only window", func() {
				sinceVal := 5 * time.Minute
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "big-step-no-until",
							PromQuery: "up",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 10 * time.Minute},
								Since: &TimeRef{duration: &sinceVal},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(BeEmpty())
			})

			It("should reject range when query-type is instant", func() {
				sinceVal := time.Hour
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "instant-with-range-fields",
							PromQuery: "up",
							QueryType: "instant",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 30 * time.Second},
								Since: &TimeRef{duration: &sinceVal},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("range can only be set"))
			})

			It("should require since for range queries", func() {
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "no-since",
							PromQuery: "up",
							QueryType: "range",
							Range: &RangeWindow{
								Step: &Duration{Duration: 30 * time.Second},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("range.since is required"))
			})
		})

		Context("when validating since/until timestamps for range queries", func() {
			It("should allow valid since/until with absolute timestamps", func() {
				start := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
				end := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "range-abs",
							PromQuery: "rate(node_cpu_seconds_total[5m])",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 30 * time.Second},
								Since: &TimeRef{absolute: &start},
								Until: &TimeRef{absolute: &end},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(BeEmpty())
			})

			It("should allow mixed since(duration) + until(absolute)", func() {
				sinceVal := 2 * time.Hour
				untilAbs := time.Now().Add(time.Hour)
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "range-mixed",
							PromQuery: "up",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 30 * time.Second},
								Since: &TimeRef{duration: &sinceVal},
								Until: &TimeRef{absolute: &untilAbs},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(BeEmpty())
			})

			It("should reject since equal to until when both are absolute", func() {
				ts := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "equal-timestamps",
							PromQuery: "up",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 30 * time.Second},
								Since: &TimeRef{absolute: &ts},
								Until: &TimeRef{absolute: &ts},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("since must be before until"))
			})

			It("should reject since after until when both are absolute", func() {
				start := time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC)
				end := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "reversed-timestamps",
							PromQuery: "up",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 30 * time.Second},
								Since: &TimeRef{absolute: &start},
								Until: &TimeRef{absolute: &end},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("since must be before until"))
			})

			It("should warn but not error when step is larger than since-until window (absolute timestamps)", func() {
				start := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
				end := time.Date(2026, 4, 7, 12, 5, 0, 0, time.UTC)
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "big-step",
							PromQuery: "up",
							QueryType: "range",
							Range: &RangeWindow{
								Step:  &Duration{Duration: 10 * time.Minute},
								Since: &TimeRef{absolute: &start},
								Until: &TimeRef{absolute: &end},
							},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(BeEmpty())
			})

			It("should reject range on instant queries", func() {
				sinceVal := time.Hour
				kpis := KPIs{
					Queries: []Query{
						{
							ID:        "instant-with-range",
							PromQuery: "up",
							QueryType: "instant",
							Range:     &RangeWindow{Since: &TimeRef{duration: &sinceVal}},
						},
					},
				}

				errors := ValidateKPIs(kpis)
				Expect(errors).To(HaveLen(1))
				Expect(errors[0].Error()).To(ContainSubstring("range can only be set when query-type is 'range'"))
			})

			It("should parse since/until from RFC 3339 format in JSON", func() {
				kpisJSON := `{
					"kpis": [
						{
							"id": "ts-range",
							"promquery": "up",
							"query-type": "range",
							"range": {
								"step": "30s",
								"since": "2026-04-07T12:24:25Z",
								"until": "2026-04-08T22:34:25Z"
							}
						}
					]
				}`
				kpisPath := filepath.Join(tmpDir, "ts.json")
				err := os.WriteFile(kpisPath, []byte(kpisJSON), 0644)
				Expect(err).NotTo(HaveOccurred())

				kpis, err := LoadKPIs(kpisPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(kpis.Queries).To(HaveLen(1))
				Expect(kpis.Queries[0].Range).NotTo(BeNil())
				Expect(kpis.Queries[0].Range.Since).NotTo(BeNil())
				Expect(kpis.Queries[0].Range.Until).NotTo(BeNil())
				Expect(kpis.Queries[0].Range.Since.IsAbsolute()).To(BeTrue())
				Expect(kpis.Queries[0].Range.Since.AbsoluteValue().Year()).To(Equal(2026))
				Expect(kpis.Queries[0].Range.Since.AbsoluteValue().Month()).To(Equal(time.April))
				Expect(kpis.Queries[0].Range.Since.AbsoluteValue().Day()).To(Equal(7))
				Expect(kpis.Queries[0].Range.Until.AbsoluteValue().Day()).To(Equal(8))
			})

			It("should parse since/until with timezone offset in JSON", func() {
				kpisJSON := `{
					"kpis": [
						{
							"id": "ts-range-tz",
							"promquery": "up",
							"query-type": "range",
							"range": {
								"step": "1m",
								"since": "2026-04-07T14:24:25+02:00",
								"until": "2026-04-08T22:34:25+02:00"
							}
						}
					]
				}`
				kpisPath := filepath.Join(tmpDir, "ts_tz.json")
				err := os.WriteFile(kpisPath, []byte(kpisJSON), 0644)
				Expect(err).NotTo(HaveOccurred())

				kpis, err := LoadKPIs(kpisPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(kpis.Queries).To(HaveLen(1))
				Expect(kpis.Queries[0].Range.Since).NotTo(BeNil())
				Expect(kpis.Queries[0].Range.Since.AbsoluteValue().Hour()).To(Equal(14))
				Expect(kpis.Queries[0].Range.Until.AbsoluteValue().Hour()).To(Equal(22))
			})

			It("should parse since as duration in JSON", func() {
				kpisJSON := `{
					"kpis": [
						{
							"id": "since-range",
							"promquery": "up",
							"query-type": "range",
							"range": {
								"step": "30s",
								"since": "1h"
							}
						}
					]
				}`
				kpisPath := filepath.Join(tmpDir, "since.json")
				err := os.WriteFile(kpisPath, []byte(kpisJSON), 0644)
				Expect(err).NotTo(HaveOccurred())

				kpis, err := LoadKPIs(kpisPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(kpis.Queries).To(HaveLen(1))
				Expect(kpis.Queries[0].Range).NotTo(BeNil())
				Expect(kpis.Queries[0].Range.Since).NotTo(BeNil())
				Expect(kpis.Queries[0].Range.Since.IsDuration()).To(BeTrue())
				Expect(kpis.Queries[0].Range.Since.DurationValue()).To(Equal(time.Hour))
			})

			It("should parse mixed since(duration) + until(RFC3339) in JSON", func() {
				kpisJSON := `{
					"kpis": [
						{
							"id": "mixed-range",
							"promquery": "up",
							"query-type": "range",
							"range": {
								"step": "30s",
								"since": "2h",
								"until": "2026-04-12T23:20:50Z"
							}
						}
					]
				}`
				kpisPath := filepath.Join(tmpDir, "mixed.json")
				err := os.WriteFile(kpisPath, []byte(kpisJSON), 0644)
				Expect(err).NotTo(HaveOccurred())

				kpis, err := LoadKPIs(kpisPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(kpis.Queries).To(HaveLen(1))
				Expect(kpis.Queries[0].Range.Since.IsDuration()).To(BeTrue())
				Expect(kpis.Queries[0].Range.Since.DurationValue()).To(Equal(2 * time.Hour))
				Expect(kpis.Queries[0].Range.Until.IsAbsolute()).To(BeTrue())
				Expect(kpis.Queries[0].Range.Until.AbsoluteValue().Year()).To(Equal(2026))
			})

			It("should reject non-RFC-3339 and non-duration format in JSON", func() {
				kpisJSON := `{
					"kpis": [
						{
							"id": "bad-ts",
							"promquery": "up",
							"query-type": "range",
							"range": {
								"step": "30s",
								"since": "Tue Apr  7 12:24:25 PM CEST 2026"
							}
						}
					]
				}`
				kpisPath := filepath.Join(tmpDir, "badts.json")
				err := os.WriteFile(kpisPath, []byte(kpisJSON), 0644)
				Expect(err).NotTo(HaveOccurred())

				_, err = LoadKPIs(kpisPath)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to decode kpis file"))
			})
		})
	})

	Describe("validateTimestampPositive", func() {
		It("should return nil for a positive duration", func() {
			d := 2 * time.Hour
			timeRef := &TimeRef{duration: &d}

			err := validateTimeRefPositive("my-kpi", "since", timeRef)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error for a zero duration", func() {
			d := time.Duration(0)
			timeRef := &TimeRef{duration: &d}

			err := validateTimeRefPositive("my-kpi", "since", timeRef)
			Expect(err).To(MatchError("KPI 'my-kpi': range.since must be > 0 when specified as a duration"))
		})

		It("should return an error for a negative duration", func() {
			d := -30 * time.Minute
			timeRef := &TimeRef{duration: &d}

			err := validateTimeRefPositive("neg-kpi", "until", timeRef)
			Expect(err).To(MatchError("KPI 'neg-kpi': range.until must be > 0 when specified as a duration"))
		})

		It("should return nil for an absolute timestamp", func() {
			abs := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
			timeRef := &TimeRef{absolute: &abs}

			err := validateTimeRefPositive("abs-kpi", "since", timeRef)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include the correct field name in the error message", func() {
			d := time.Duration(0)
			timeRef := &TimeRef{duration: &d}

			errSince := validateTimeRefPositive("kpi-x", "since", timeRef)
			Expect(errSince).To(MatchError("KPI 'kpi-x': range.since must be > 0 when specified as a duration"))

			errUntil := validateTimeRefPositive("kpi-x", "until", timeRef)
			Expect(errUntil).To(MatchError("KPI 'kpi-x': range.until must be > 0 when specified as a duration"))
		})
	})
})
