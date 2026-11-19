package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
	kpiprofiles "github.com/redhat-best-practices-for-k8s/kpi-collection-tool/prom-kpi-profiles"
	taskprofiles "github.com/redhat-best-practices-for-k8s/kpi-collection-tool/task-profiles"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"
)

type domain struct {
	Name string
	IDs  []string
}

type profile struct {
	Name    string
	File    string
	Domains []domain
}

type selectedTasks struct {
	prometheus, perNode, oslat, recovery bool
}

func (s selectedTasks) any() bool {
	return s.prometheus || s.perNode || s.oslat || s.recovery
}

func (s selectedTasks) order() []string {
	var out []string
	for _, name := range config.KnownTaskConfigs {
		if s.selected(name) {
			out = append(out, name)
		}
	}
	return out
}

func (s selectedTasks) selected(name string) bool {
	switch name {
	case config.TaskConfigPrometheus:
		return s.prometheus
	case config.TaskConfigPerNodeData:
		return s.perNode
	case config.TaskConfigOslat:
		return s.oslat
	case config.TaskConfigAppRecoveryTime:
		return s.recovery
	default:
		return false
	}
}

var profiles = map[string]profile{
	"ran":  ranProfile(),
	"core": coreProfile(),
	"hub":  hubProfile(),
}

var kpisGenerateFlags struct {
	profile       string
	file          string
	all           bool
	overwrite     bool
	uncategorized bool
}

var kpisGenerateCmd = &cobra.Command{
	Use:   "generate --profile <profile>",
	Short: "Generate KPI and task configuration for a cluster profile",
	Long: `Generate a tasks.yaml for a cluster profile (ran, core, hub).

Always prompts for which tasks to include ([task]). If prometheus is selected,
prompts for KPI domains ([kpi]) unless --all is set (then all domains are included).
App-recovery-time asks for confirmation on a TTY (it reboots nodes).

Output is always one inline tasks.yaml (default name: tasks.yaml). Edit CHANGE_ME
placeholders before running; see docs for per-task guidance. Use --uncategorized
to omit KPI category fields.`,
	Example: `  # Interactively pick tasks; --all includes every KPI domain for prometheus
  kpi-collector kpis generate --profile ran --all

  # Custom output path
  kpi-collector kpis generate --profile core -f core-tasks.yaml

  # Uncategorized KPIs inside the tasks file
  kpi-collector kpis generate --profile ran --all --uncategorized`,
	Args: cobra.NoArgs,
	RunE: runKpisGenerate,
}

func init() {
	kpisCmd.AddCommand(kpisGenerateCmd)

	kpisGenerateCmd.Flags().StringVarP(&kpisGenerateFlags.profile, "profile", "p", "",
		fmt.Sprintf("cluster profile (%s)", strings.Join(config.ValidClusterTypes, ", ")))
	kpisGenerateCmd.Flags().StringVarP(&kpisGenerateFlags.file, "file", "f", "",
		"output path (default: tasks.yaml)")
	kpisGenerateCmd.Flags().BoolVar(&kpisGenerateFlags.all, "all", false,
		"include all Prometheus KPI domains without domain prompts (tasks are still prompted)")
	kpisGenerateCmd.Flags().BoolVar(&kpisGenerateFlags.overwrite, "overwrite", false,
		"overwrite the output file if it already exists")
	kpisGenerateCmd.Flags().BoolVar(&kpisGenerateFlags.uncategorized, "uncategorized", false,
		"generate KPIs without category fields (all data stored in a single table)")

	_ = kpisGenerateCmd.MarkFlagRequired("profile")
}

func runKpisGenerate(_ *cobra.Command, _ []string) error {
	profileName := kpisGenerateFlags.profile

	prof, ok := profiles[profileName]
	if !ok {
		return fmt.Errorf("unknown profile %q, valid profiles: %s",
			profileName, strings.Join(config.ValidClusterTypes, ", "))
	}

	outPath := resolveOutputPath()
	if err := validateOutputPath(outPath, kpisGenerateFlags.overwrite); err != nil {
		return fmt.Errorf("failed to validate output path: %w", err)
	}

	reader := bufio.NewReader(os.Stdin)
	sel, err := selectTasks(reader, stdinIsTTY())
	if err != nil {
		return fmt.Errorf("failed to select tasks: %w", err)
	}
	if !sel.any() {
		fmt.Println("No tasks selected. No file generated.")
		return nil
	}

	var queries []config.Query
	if sel.prometheus {
		queries, err = selectPrometheusQueries(prof, reader)
		if err != nil {
			return err
		}
		if len(queries) == 0 {
			fmt.Println("No KPI domains selected.")
			sel.prometheus = false
			if !sel.any() {
				fmt.Println("No file generated.")
				return nil
			}
		}
		if kpisGenerateFlags.uncategorized {
			queries = stripCategories(queries)
		}
	}

	return writeTasksFile(outPath, sel, queries)
}

func resolveOutputPath() string {
	if kpisGenerateFlags.file != "" {
		return kpisGenerateFlags.file
	}
	return "tasks.yaml"
}

func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func selectTasks(reader *bufio.Reader, interactive bool) (selectedTasks, error) {
	fmt.Println("--- Tasks to include ---")
	var sel selectedTasks
	var err error
	for _, p := range []struct {
		dst  *bool
		name string
	}{
		{&sel.prometheus, "prometheus"},
		{&sel.perNode, "per-node-data"},
		{&sel.oslat, "oslat"},
	} {
		if *p.dst, err = promptYesNo(reader, "[task] "+p.name); err != nil {
			return sel, err
		}
	}
	if !interactive {
		return sel, nil
	}
	fmt.Println("WARNING: app-recovery-time reboots the listed node(s). Use only on disposable lab workers.")
	sel.recovery, err = promptYesNo(reader, "[task] app-recovery-time")
	return sel, err
}

func selectPrometheusQueries(prof profile, reader *bufio.Reader) ([]config.Query, error) {
	allKPIs, err := loadProfileKPIs(prof)
	if err != nil {
		return nil, fmt.Errorf("failed to load profile KPIs: %w", err)
	}
	if kpisGenerateFlags.all {
		return allKPIs.Queries, nil
	}
	fmt.Println("\n--- Prometheus KPI domains ---")
	return promptForDomains(reader, prof, allKPIs.Queries)
}

func writeTasksFile(path string, sel selectedTasks, queries []config.Query) error {
	if err := writeInlineTasks(path, sel, queries); err != nil {
		return err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	fmt.Printf("Generated %s\n", absPath)
	fmt.Println("Edit CHANGE_ME placeholders before running (see docs for task guidance).")
	return nil
}

func writeInlineTasks(path string, sel selectedTasks, queries []config.Query) error {
	spec, err := buildGenerateTasksSpec(sel, queries)
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(&spec)
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func buildGenerateTasksSpec(sel selectedTasks, queries []config.Query) (config.TasksSpec, error) {
	spec := config.TasksSpec{
		Orchestration: config.OrchestrationConfig{
			Mode:      config.ModeSequential,
			OnFailure: config.OnFailureFailFast,
			Order:     sel.order(),
		},
	}
	if sel.prometheus {
		spec.Prometheus = &config.PrometheusTaskConfig{Kpis: queries}
	}
	if sel.perNode {
		cfg, err := loadTaskFragment[config.PerNodeDataTaskConfig](
			taskprofiles.PerNodeDataFull, config.TaskConfigPerNodeData)
		if err != nil {
			return config.TasksSpec{}, err
		}
		spec.PerNodeData = cfg
	}
	if sel.oslat {
		cfg, err := loadTaskFragment[config.OslatTaskConfig](
			taskprofiles.OslatFull, config.TaskConfigOslat)
		if err != nil {
			return config.TasksSpec{}, err
		}
		spec.Oslat = cfg
	}
	if sel.recovery {
		cfg, err := loadTaskFragment[config.AppRecoveryTimeTaskConfig](
			taskprofiles.AppRecoveryTimeFull, config.TaskConfigAppRecoveryTime)
		if err != nil {
			return config.TasksSpec{}, err
		}
		spec.AppRecoveryTime = cfg
	}
	return spec, nil
}

func loadTaskFragment[T any](frag, rootKey string) (*T, error) {
	data, err := taskprofiles.FS.ReadFile(frag)
	if err != nil {
		return nil, fmt.Errorf("read embedded %s: %w", frag, err)
	}
	var wrap map[string]*T
	if err := yaml.Unmarshal(data, &wrap); err != nil {
		return nil, fmt.Errorf("parse embedded %s: %w", frag, err)
	}
	cfg := wrap[rootKey]
	if cfg == nil {
		return nil, fmt.Errorf("embedded %s missing %q root key", frag, rootKey)
	}
	return cfg, nil
}

func validateOutputPath(filePath string, overwrite bool) error {
	if info, err := os.Stat(filePath); err == nil {
		if info.IsDir() {
			return fmt.Errorf("%q is a directory, not a file path", filePath)
		}
		if !overwrite {
			return fmt.Errorf("file %q already exists (use --overwrite to replace it)", filePath)
		}
	}

	dir := filepath.Dir(filePath)
	if dir == "." {
		return nil
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("directory %q does not exist", dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", dir)
	}
	return nil
}

func stripCategories(queries []config.Query) []config.Query {
	stripped := make([]config.Query, len(queries))
	copy(stripped, queries)
	for i := range stripped {
		stripped[i].Category = ""
	}
	return stripped
}

func loadProfileKPIs(prof profile) (config.KPIs, error) {
	data, err := kpiprofiles.FS.ReadFile(prof.File)
	if err != nil {
		return config.KPIs{}, fmt.Errorf("failed to read embedded profile %q: %w", prof.File, err)
	}
	var kpis config.KPIs
	if err := yaml.Unmarshal(data, &kpis); err != nil {
		return config.KPIs{}, fmt.Errorf("failed to parse profile %q: %w", prof.File, err)
	}
	return kpis, nil
}

func promptForDomains(reader *bufio.Reader, prof profile, allQueries []config.Query) ([]config.Query, error) {
	var selectedDomains []domain
	for _, d := range prof.Domains {
		selected, err := promptYesNo(reader, fmt.Sprintf("[kpi] %s", d.Name))
		if err != nil {
			return nil, fmt.Errorf("failed to read input: %w", err)
		}
		if selected {
			selectedDomains = append(selectedDomains, d)
		}
	}
	return filterByDomains(allQueries, selectedDomains), nil
}

func filterByDomains(allQueries []config.Query, domains []domain) []config.Query {
	allowed := make(map[string]bool)
	for _, d := range domains {
		for _, id := range d.IDs {
			allowed[id] = true
		}
	}
	var filtered []config.Query
	for _, q := range allQueries {
		if allowed[q.ID] {
			filtered = append(filtered, q)
		}
	}
	return filtered
}

func promptYesNo(reader *bufio.Reader, question string) (bool, error) {
	for {
		fmt.Printf("%s [y/n]: ", question)
		input, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}
		switch strings.TrimSpace(strings.ToLower(input)) {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Println("Please answer y or n.")
		}
	}
}

func ranProfile() profile {
	return profile{
		Name: "ran",
		File: kpiprofiles.RAN,
		Domains: []domain{
			{Name: "CPU & Resource Isolation", IDs: []string{
				"cpu-reserved-cores", "cpu-isolated-cores", "cpu-node-total",
				"cpu-system-slice", "cpu-ovs-slice", "cpu-pods-average",
			}},
			{Name: "Memory & HugePages", IDs: []string{
				"memory-node-used-percentage", "memory-working-set-by-pod", "memory-rss-by-pod",
				"hugepages-1g-free", "hugepages-1g-total", "hugepages-2m-free", "hugepages-2m-total",
			}},
			{Name: "PTP / Timing", IDs: []string{
				"ptp-offset-master", "ptp-max-offset-master",
				"ptp-clock-state", "ptp-interface-role",
			}},
			{Name: "Networking & OVN", IDs: []string{
				"network-node-rx-bytes", "network-node-tx-bytes",
				"network-node-rx-errors", "network-node-tx-errors",
				"network-container-rx-bytes", "network-container-tx-bytes",
				"ovn-controller-cpu", "ovn-controller-memory",
			}},
			{Name: "System & Storage", IDs: []string{
				"disk-io-read-bytes", "disk-io-write-bytes", "disk-usage-percentage",
				"node-context-switches", "node-interrupts", "pod-restart-count",
			}},
		},
	}
}

func coreProfile() profile {
	return profile{
		Name: "core",
		File: kpiprofiles.Core,
		Domains: []domain{
			{Name: "Control Plane & etcd", IDs: []string{
				"apiserver-request-duration-99p", "apiserver-request-rate", "apiserver-error-rate",
				"etcd-db-size", "etcd-disk-wal-fsync-duration-99p", "etcd-leader-changes",
			}},
			{Name: "Node & Pod Resources", IDs: []string{
				"cpu-node-total", "cpu-usage-by-namespace",
				"memory-node-used-percentage", "memory-usage-by-namespace",
				"node-load-1min", "node-load-5min",
				"pod-restart-count", "pod-status-not-ready", "cluster-uptime",
			}},
			{Name: "Networking & Ingress", IDs: []string{
				"network-node-rx-bytes", "network-node-tx-bytes", "ingress-request-rate",
			}},
			{Name: "Storage", IDs: []string{
				"disk-usage-percentage", "pv-usage-percentage",
				"disk-io-read-bytes", "disk-io-write-bytes",
			}},
		},
	}
}

func hubProfile() profile {
	return profile{
		Name: "hub",
		File: kpiprofiles.Hub,
		Domains: []domain{
			{Name: "Control Plane & etcd", IDs: []string{
				"apiserver-request-duration-99p", "apiserver-request-rate", "apiserver-error-rate",
				"etcd-db-size", "etcd-disk-wal-fsync-duration-99p", "etcd-leader-changes",
			}},
			{Name: "ACM & GitOps", IDs: []string{
				"acm-cpu-usage", "acm-memory-usage",
				"gitops-cpu-usage", "gitops-memory-usage",
				"acm-managed-clusters", "acm-policy-noncompliant",
			}},
			{Name: "Node & Pod Resources", IDs: []string{
				"cpu-node-total", "memory-node-used-percentage", "node-load-1min",
				"pod-restart-count", "pod-status-not-ready", "cluster-uptime",
			}},
			{Name: "Networking & Storage", IDs: []string{
				"network-node-rx-bytes", "network-node-tx-bytes",
				"disk-usage-percentage", "pv-usage-percentage",
			}},
		},
	}
}
