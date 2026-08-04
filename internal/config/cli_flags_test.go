package config

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	validClusterName  = "test-cluster"
	validClusterType  = "ran"
	validBearerToken  = "test-token"
	validThanosURL    = "https://thanos.example.com"
	validKubeconfig   = "/path/to/kubeconfig"
	validSamplingFreq = 60 * time.Second
	validDuration     = 45 * time.Minute
	validDatabaseType = "sqlite"
	validPromKPIsConfig = "/path/to/kpis.yaml"

	errClusterNameRequiredMsg = "cluster name is required: use --cluster-name flag"
	errClusterTypeRequiredMsg = "cluster-type is required: must be 'ran', 'core', or 'hub'"
	errInvalidClusterTypeMsg  = "invalid cluster-type 'invalid': must be 'ran', 'core', or 'hub'"
	errInvalidFlagComboMsg    = "invalid flag combination: either provide --token and --thanos-url, or provide --kubeconfig"
	errSamplingFreqMsg        = "sampling frequency must be greater than 0"
	errDurationMsg            = "duration must be greater than 0"
	errInvalidDBTypeMsg       = "invalid db-type: must be 'sqlite' or 'postgres'"
	errPostgresURLRequiredMsg = "postgres-url is required when db-type=postgres"
	errNoTaskConfigMsg = "at least one task config flag is required (--prom-kpis-config, --oslat-config, --per-node-config, or --recovery-config)"
)

var _ = Describe("validateFlags test", func() {

	DescribeTable("flag validation scenarios",
		func(flags InputFlags, expectedErr string) {
			err := ValidateFlags(flags)

			if expectedErr != "" {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expectedErr))
			} else {
				Expect(err).ToNot(HaveOccurred())
			}
		},

		// Valid cases
		Entry("valid token and thanos-url",
			InputFlags{
				ClusterName:  validClusterName,
				ClusterType:  validClusterType,
				BearerToken:  validBearerToken,
				ThanosURL:    validThanosURL,
				SamplingFreq: validSamplingFreq,
				Duration:     validDuration,
				DatabaseType: validDatabaseType,
				PromKPIsConfig: validPromKPIsConfig,
			},
			"", // no error expected
		),
		Entry("valid kubeconfig",
			InputFlags{
				ClusterName:  validClusterName,
				ClusterType:  validClusterType,
				Kubeconfig:   validKubeconfig,
				SamplingFreq: validSamplingFreq,
				Duration:     validDuration,
				DatabaseType: validDatabaseType,
				PromKPIsConfig: validPromKPIsConfig,
			},
			"",
		),
		// Error cases - missing cluster name
		Entry("missing cluster name",
			InputFlags{
				ClusterType: validClusterType,
				BearerToken: validBearerToken,
				ThanosURL:   validThanosURL,
			},
			errClusterNameRequiredMsg,
		),
		// Error cases - missing or invalid cluster type
		Entry("missing cluster type",
			InputFlags{
				ClusterName:  validClusterName,
				BearerToken:  validBearerToken,
				ThanosURL:    validThanosURL,
				SamplingFreq: validSamplingFreq,
				Duration:     validDuration,
				DatabaseType: validDatabaseType,
				PromKPIsConfig: validPromKPIsConfig,
			},
			errClusterTypeRequiredMsg,
		),
		Entry("invalid cluster type",
			InputFlags{
				ClusterName:  validClusterName,
				ClusterType:  "invalid",
				BearerToken:  validBearerToken,
				ThanosURL:    validThanosURL,
				SamplingFreq: validSamplingFreq,
				Duration:     validDuration,
				DatabaseType: validDatabaseType,
				PromKPIsConfig: validPromKPIsConfig,
			},
			errInvalidClusterTypeMsg,
		),
		// Error cases - invalid flag combinations
		Entry("only token provided",
			InputFlags{
				ClusterName: validClusterName,
				ClusterType: validClusterType,
				BearerToken: validBearerToken,
			},
			errInvalidFlagComboMsg,
		),
		Entry("only thanos-url provided",
			InputFlags{
				ClusterName: validClusterName,
				ClusterType: validClusterType,
				ThanosURL:   validThanosURL,
			},
			errInvalidFlagComboMsg,
		),
		Entry("all three auth methods provided",
			InputFlags{
				ClusterName: validClusterName,
				ClusterType: validClusterType,
				BearerToken: validBearerToken,
				ThanosURL:   validThanosURL,
				Kubeconfig:  validKubeconfig,
			},
			errInvalidFlagComboMsg,
		),
		Entry("no authentication method",
			InputFlags{
				ClusterName: validClusterName,
				ClusterType: validClusterType,
			},
			errInvalidFlagComboMsg,
		),
		Entry("token and kubeconfig without thanos-url",
			InputFlags{
				ClusterName: validClusterName,
				ClusterType: validClusterType,
				BearerToken: validBearerToken,
				Kubeconfig:  validKubeconfig,
			},
			errInvalidFlagComboMsg,
		),
		Entry("thanos-url and kubeconfig without token",
			InputFlags{
				ClusterName: validClusterName,
				ClusterType: validClusterType,
				ThanosURL:   validThanosURL,
				Kubeconfig:  validKubeconfig,
			},
			errInvalidFlagComboMsg,
		),
		// Error cases - invalid sampling frequency
		Entry("zero sampling frequency",
			InputFlags{
				ClusterName:  validClusterName,
				ClusterType:  validClusterType,
				BearerToken:  validBearerToken,
				ThanosURL:    validThanosURL,
				SamplingFreq: 0,
				Duration:     validDuration,
				DatabaseType: validDatabaseType,
				PromKPIsConfig: validPromKPIsConfig,
			},
			errSamplingFreqMsg,
		),
		Entry("negative sampling frequency",
			InputFlags{
				ClusterName:  validClusterName,
				ClusterType:  validClusterType,
				BearerToken:  validBearerToken,
				ThanosURL:    validThanosURL,
				SamplingFreq: -10,
				Duration:     validDuration,
				DatabaseType: validDatabaseType,
				PromKPIsConfig: validPromKPIsConfig,
			},
			errSamplingFreqMsg,
		),
		// Error cases - invalid duration
		Entry("zero duration",
			InputFlags{
				ClusterName:  validClusterName,
				ClusterType:  validClusterType,
				BearerToken:  validBearerToken,
				ThanosURL:    validThanosURL,
				SamplingFreq: validSamplingFreq,
				Duration:     0,
				DatabaseType: validDatabaseType,
				PromKPIsConfig: validPromKPIsConfig,
			},
			errDurationMsg,
		),
		Entry("negative duration",
			InputFlags{
				ClusterName:  validClusterName,
				ClusterType:  validClusterType,
				BearerToken:  validBearerToken,
				ThanosURL:    validThanosURL,
				SamplingFreq: validSamplingFreq,
				Duration:     -10 * time.Minute,
				DatabaseType: validDatabaseType,
				PromKPIsConfig: validPromKPIsConfig,
			},
			errDurationMsg,
		),
		// Error cases - invalid database type
		Entry("invalid database type",
			InputFlags{
				ClusterName:  validClusterName,
				ClusterType:  validClusterType,
				BearerToken:  validBearerToken,
				ThanosURL:    validThanosURL,
				SamplingFreq: validSamplingFreq,
				Duration:     validDuration,
				DatabaseType: "mysql",
				PromKPIsConfig: validPromKPIsConfig,
			},
			errInvalidDBTypeMsg,
		),
		Entry("empty database type",
			InputFlags{
				ClusterName:  validClusterName,
				ClusterType:  validClusterType,
				BearerToken:  validBearerToken,
				ThanosURL:    validThanosURL,
				SamplingFreq: validSamplingFreq,
				Duration:     validDuration,
				DatabaseType: "",
				PromKPIsConfig: validPromKPIsConfig,
			},
			errInvalidDBTypeMsg,
		),
		// Error cases - postgres without URL
		Entry("postgres database type without postgres-url",
			InputFlags{
				ClusterName:  validClusterName,
				ClusterType:  validClusterType,
				BearerToken:  validBearerToken,
				ThanosURL:    validThanosURL,
				SamplingFreq: validSamplingFreq,
				Duration:     validDuration,
				DatabaseType: "postgres",
				PostgresURL:  "",
				PromKPIsConfig: validPromKPIsConfig,
			},
			errPostgresURLRequiredMsg,
		),
		// Valid case - postgres with URL
		Entry("valid postgres with postgres-url",
			InputFlags{
				ClusterName:  validClusterName,
				ClusterType:  validClusterType,
				BearerToken:  validBearerToken,
				ThanosURL:    validThanosURL,
				SamplingFreq: validSamplingFreq,
				Duration:     validDuration,
				DatabaseType: "postgres",
				PostgresURL:  "postgresql://user:pass@localhost:5432/dbname",
				PromKPIsConfig: validPromKPIsConfig,
			},
			"",
		),
		// Valid case - single-run mode
		Entry("valid single-run mode",
			InputFlags{
				ClusterName:  validClusterName,
				ClusterType:  validClusterType,
				BearerToken:  validBearerToken,
				ThanosURL:    validThanosURL,
				SingleRun:    true,
				SamplingFreq: validSamplingFreq,
				Duration:     validDuration,
				DatabaseType: validDatabaseType,
				PromKPIsConfig: validPromKPIsConfig,
			},
			"",
		),
		// Error cases - no task config flag set
		Entry("no task config flags",
			InputFlags{
				ClusterName:  validClusterName,
				ClusterType:  validClusterType,
				BearerToken:  validBearerToken,
				ThanosURL:    validThanosURL,
				SamplingFreq: validSamplingFreq,
				Duration:     validDuration,
				DatabaseType: validDatabaseType,
				PromKPIsConfig: "",
			},
			errNoTaskConfigMsg,
		),
	)
})
