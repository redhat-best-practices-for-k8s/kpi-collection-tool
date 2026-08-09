package task

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"

	"gopkg.in/yaml.v3"
)

const (
	ModeSequential = "sequential"
	ModeParallel   = "parallel"

	OnFailureContinue = "continue"
	OnFailureFailFast = "fail-fast"

	bundleFileName = "bundle.yaml"
)

// BundleSpec describes a playlist of tasks and how to execute them.
type BundleSpec struct {
	Tasks     []BundleTaskRef `yaml:"tasks"`
	Mode      string          `yaml:"mode"`
	OnFailure string          `yaml:"on-failure"`

	// baseDir is the directory containing the bundle file, used to resolve
	// relative task file paths.
	baseDir string
}

// BundleTaskRef is a reference to a single task file inside a bundle.
type BundleTaskRef struct {
	File string `yaml:"file"`
	Type string `yaml:"type"`
}

// LoadBundle reads a bundle from a file path or a directory containing bundle.yaml.
func LoadBundle(path string) (BundleSpec, error) {
	info, err := os.Stat(path)
	if err != nil {
		return BundleSpec{}, fmt.Errorf("cannot access bundle path %q: %w", path, err)
	}

	bundlePath := path
	if info.IsDir() {
		bundlePath = filepath.Join(path, bundleFileName)
	}

	data, err := os.ReadFile(bundlePath) //#nosec G304 -- path is user-provided CLI input
	if err != nil {
		return BundleSpec{}, fmt.Errorf("failed to read bundle file %q: %w", bundlePath, err)
	}

	var spec BundleSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return BundleSpec{}, fmt.Errorf("failed to parse bundle file %q: %w", bundlePath, err)
	}

	spec.baseDir = filepath.Dir(bundlePath)

	if err := validateBundleSpec(&spec); err != nil {
		return BundleSpec{}, err
	}

	return spec, nil
}

func validateBundleSpec(spec *BundleSpec) error {
	if len(spec.Tasks) == 0 {
		return fmt.Errorf("bundle has no tasks defined")
	}

	if spec.Mode == "" {
		spec.Mode = ModeSequential
	}
	if spec.Mode != ModeSequential && spec.Mode != ModeParallel {
		return fmt.Errorf("invalid bundle mode %q: must be %q or %q", spec.Mode, ModeSequential, ModeParallel)
	}

	if spec.OnFailure == "" {
		spec.OnFailure = OnFailureContinue
	}
	if spec.OnFailure != OnFailureContinue && spec.OnFailure != OnFailureFailFast {
		return fmt.Errorf("invalid bundle on-failure %q: must be %q or %q", spec.OnFailure, OnFailureContinue, OnFailureFailFast)
	}

	for i, ref := range spec.Tasks {
		if ref.File == "" {
			return fmt.Errorf("bundle task #%d has no file specified", i+1)
		}
	}

	return nil
}

// ResolveBundleTasks turns a BundleSpec into a list of executable Tasks.
// Only prom-kpis is supported today; other types return an error.
func ResolveBundleTasks(spec BundleSpec, flags config.InputFlags, kpis config.KPIs) ([]Task, error) {
	var tasks []Task

	for _, ref := range spec.Tasks {
		taskType := ref.Type
		if taskType == "" {
			taskType = inferTypeFromFilename(ref.File)
		}

		taskFile := ref.File
		if !filepath.IsAbs(taskFile) {
			taskFile = filepath.Join(spec.baseDir, taskFile)
		}

		switch taskType {
		case "prom-kpis":
			promKPIs, err := config.LoadKPIs(taskFile)
			if err != nil {
				return nil, fmt.Errorf("bundle task %q: failed to load KPIs: %w", ref.File, err)
			}
			tasks = append(tasks, NewPromKPITask(promKPIs, flags))

		// TODO: implement oslat, per-node, recovery task types
		default:
			return nil, fmt.Errorf("bundle task %q: task type %q is not yet supported", ref.File, taskType)
		}
	}

	return tasks, nil
}

func inferTypeFromFilename(filename string) string {
	switch base := filepath.Base(filename); base {
	case "kpis.yaml", "kpis.yml":
		return "prom-kpis"
	case "oslat.yaml", "oslat.yml":
		return "oslat"
	case "per-node.yaml", "per-node.yml":
		return "per-node"
	case "recovery.yaml", "recovery.yml":
		return "recovery"
	default:
		return ""
	}
}
