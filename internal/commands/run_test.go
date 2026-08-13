package commands

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
)

var _ = Describe("mergeYAMLConfigIntoFlags", func() {
	// cliFreq/cliDuration represent values already sitting in flags because the
	// user explicitly passed --frequency/--duration; they intentionally differ
	// from the YAML values below so a wrong merge is easy to spot.
	var (
		cliFreq      = 10 * time.Second
		cliDuration  = 5 * time.Minute
		yamlFreq     = config.Duration{Duration: 30 * time.Second}
		yamlDuration = config.Duration{Duration: 2 * time.Hour}
		yamlOnce     = true

		cfg   *config.PromConfig
		flags *config.InputFlags
	)

	BeforeEach(func() {
		cfg = &config.PromConfig{
			Frequency: &yamlFreq,
			Duration:  &yamlDuration,
			Once:      &yamlOnce,
		}
		flags = &config.InputFlags{
			SamplingFreq: cliFreq,
			Duration:     cliDuration,
			SingleRun:    false,
		}
	})

	allFlagsChanged := func(string) bool { return true }
	noFlagsChanged := func(string) bool { return false }

	It("keeps the CLI value when the corresponding flag was explicitly set", func() {
		mergeYAMLConfigIntoFlags(cfg, allFlagsChanged, flags)

		Expect(flags.SamplingFreq).To(Equal(cliFreq), "CLI-set frequency must win over YAML")
		Expect(flags.Duration).To(Equal(cliDuration), "CLI-set duration must win over YAML")
		Expect(flags.SingleRun).To(BeFalse(), "CLI-set once must win over YAML")
	})

	It("falls back to the YAML value when the CLI flag was not set", func() {
		mergeYAMLConfigIntoFlags(cfg, noFlagsChanged, flags)

		Expect(flags.SamplingFreq).To(Equal(yamlFreq.Duration), "YAML frequency should apply when CLI flag is unset")
		Expect(flags.Duration).To(Equal(yamlDuration.Duration), "YAML duration should apply when CLI flag is unset")
		Expect(flags.SingleRun).To(BeTrue(), "YAML once should apply when CLI flag is unset")
	})

	It("keeps the pre-existing default when neither the CLI flag nor YAML config set it", func() {
		mergeYAMLConfigIntoFlags(&config.PromConfig{}, noFlagsChanged, flags)

		Expect(flags.SamplingFreq).To(Equal(cliFreq))
		Expect(flags.Duration).To(Equal(cliDuration))
		Expect(flags.SingleRun).To(BeFalse())
	})

	It("is a no-op when there is no YAML config at all", func() {
		mergeYAMLConfigIntoFlags(nil, noFlagsChanged, flags)

		Expect(flags.SamplingFreq).To(Equal(cliFreq))
		Expect(flags.Duration).To(Equal(cliDuration))
		Expect(flags.SingleRun).To(BeFalse())
	})
})
