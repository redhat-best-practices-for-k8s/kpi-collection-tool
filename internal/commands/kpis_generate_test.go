package commands

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
)

func TestSelectTasksNonTTYSkipsRecovery(t *testing.T) {
	input := strings.Join([]string{"y", "y", "y"}, "\n") + "\n"
	sel, err := selectTasks(bufio.NewReader(strings.NewReader(input)), false)
	if err != nil {
		t.Fatal(err)
	}
	if !sel.prometheus || !sel.perNode || !sel.oslat {
		t.Fatalf("expected safe tasks included, got %+v", sel)
	}
	if sel.recovery {
		t.Fatal("expected recovery skipped on non-TTY")
	}
}

func TestSelectTasksTTYPromptsRecovery(t *testing.T) {
	input := strings.Join([]string{"y", "n", "n", "y"}, "\n") + "\n"
	sel, err := selectTasks(bufio.NewReader(strings.NewReader(input)), true)
	if err != nil {
		t.Fatal(err)
	}
	if !sel.prometheus || sel.perNode || sel.oslat || !sel.recovery {
		t.Fatalf("unexpected selection %+v", sel)
	}

	input = strings.Join([]string{"y", "n", "n", "n"}, "\n") + "\n"
	sel, err = selectTasks(bufio.NewReader(strings.NewReader(input)), true)
	if err != nil {
		t.Fatal(err)
	}
	if sel.recovery {
		t.Fatal("expected recovery skipped after no")
	}
}

func TestSelectTasksInteractive(t *testing.T) {
	input := strings.Join([]string{"y", "n", "y", "n"}, "\n") + "\n"
	sel, err := selectTasks(bufio.NewReader(strings.NewReader(input)), true)
	if err != nil {
		t.Fatal(err)
	}
	if !sel.prometheus || sel.perNode || !sel.oslat || sel.recovery {
		t.Fatalf("unexpected selection %+v", sel)
	}
}

func TestWriteInlineTasksMarshalsTasksSpec(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.yaml")
	queries := []config.Query{{ID: "cpu-node-total", PromQuery: "up", Category: "cpu"}}

	err := writeInlineTasks(path, selectedTasks{prometheus: true, perNode: true, oslat: true}, queries)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"orchestration:",
		"order:",
		"prometheus:",
		"kpis:",
		"cpu-node-total",
		"per-node-data:",
		"CHANGE_ME",
		"oslat:",
		"image: CHANGE_ME",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}

	loaded, err := config.LoadTasksSpec(path)
	if err != nil {
		t.Fatalf("round-trip LoadTasksSpec: %v\n%s", err, text)
	}
	if loaded.Prometheus == nil || len(loaded.Prometheus.Kpis) != 1 {
		t.Fatalf("expected prometheus KPIs, got %+v", loaded.Prometheus)
	}
	if loaded.PerNodeData == nil || loaded.Oslat == nil {
		t.Fatalf("expected per-node and oslat, got perNode=%v oslat=%v", loaded.PerNodeData, loaded.Oslat)
	}
	if loaded.AppRecoveryTime != nil {
		t.Fatal("did not select recovery")
	}
}

func TestWriteInlineTasksPrometheusOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.yaml")
	err := writeInlineTasks(path, selectedTasks{prometheus: true},
		[]config.Query{{ID: "cpu-node-total", PromQuery: "up"}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "prometheus:") || !strings.Contains(text, "cpu-node-total") {
		t.Fatalf("missing prometheus block:\n%s", text)
	}
	if strings.Contains(text, "oslat:") || strings.Contains(text, "per-node-data:") {
		t.Fatalf("unexpected non-prom tasks:\n%s", text)
	}
}
