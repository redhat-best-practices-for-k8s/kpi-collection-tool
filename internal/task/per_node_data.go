package task

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"

	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/config"
	"github.com/redhat-best-practices-for-k8s/kpi-collection-tool/internal/kubernetes"
)

const (
	meminfoMarker   = "___KPI_MEMINFO___"
	cmdlineMarker   = "___KPI_CMDLINE___"
	debugWaitBuffer = 10 * time.Minute

	// Runs inside chroot /host so top and /proc are the node's.
	collectScript = `
set -e
isol="$ISOLCPUS"
if [ -z "$isol" ]; then
  for field in $(cat /proc/cmdline); do
    case "$field" in
      isolcpus=*) isol=${field#isolcpus=} ;;
    esac
  done
  isol=$(echo "$isol" | tr ',' '\n' | grep -v '^$' | grep -v '^managed_irq$' | paste -sd, -)
fi
if [ -z "$isol" ]; then
  echo "isolcpus list is empty" >&2
  exit 1
fi
taskset -c "$isol" top -b -1 -i -w 200 -d "$TOP_DELAY" -n "$TOP_COUNT"
printf '%s\n' '` + meminfoMarker + `'
cat /proc/meminfo
printf '%s\n' '` + cmdlineMarker + `'
cat /proc/cmdline
`
)

// PerNodeDataTask runs a debug pod on every node, then writes API YAML.
type PerNodeDataTask struct {
	cfg          config.PerNodeDataTaskConfig
	kubeconfig   string
	artifactsDir string
}

// NewPerNodeDataTask constructs a per-node-data task.
func NewPerNodeDataTask(cfg config.PerNodeDataTaskConfig, kubeconfig, artifactsDir string) *PerNodeDataTask {
	return &PerNodeDataTask{cfg: cfg, kubeconfig: kubeconfig, artifactsDir: artifactsDir}
}

func (t *PerNodeDataTask) Name() string { return config.TaskConfigPerNodeData }

func (t *PerNodeDataTask) Run(ctx context.Context) error {
	client, err := kubernetes.ClientsetFromKubeconfig(t.kubeconfig)
	if err != nil {
		return fmt.Errorf("%s: %w", t.Name(), err)
	}

	nodes, err := kubernetes.ListNodeNames(ctx, client)
	if err != nil {
		return fmt.Errorf("%s: %w", t.Name(), err)
	}

	nodeErr := t.collectAll(ctx, client, nodes)
	descErr := t.writeDescribes(ctx, client)
	if nodeErr != nil {
		if descErr != nil {
			return fmt.Errorf("%s: %w; describe: %v", t.Name(), nodeErr, descErr)
		}
		return fmt.Errorf("%s: %w", t.Name(), nodeErr)
	}
	if descErr != nil {
		return fmt.Errorf("%s: %w", t.Name(), descErr)
	}
	return nil
}

func (t *PerNodeDataTask) collectAll(ctx context.Context, client *k8s.Clientset, nodes []string) error {
	var mu sync.Mutex
	var errs []error
	var wg sync.WaitGroup
	for _, node := range nodes {
		wg.Add(1)
		go func(node string) {
			defer wg.Done()
			if err := t.collectNode(ctx, client, node); err != nil {
				log.Printf("%s: %v", t.Name(), err)
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(node)
	}
	wg.Wait()
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("failed on %d node(s): %v", len(errs), errs)
}

func (t *PerNodeDataTask) collectNode(ctx context.Context, client *k8s.Clientset, node string) error {
	created, err := kubernetes.CreatePod(ctx, client, t.debugPod(node))
	if err != nil {
		return fmt.Errorf("%s: %w", node, err)
	}

	wait := t.cfg.Duration.Duration + debugWaitBuffer
	finished, waitErr := kubernetes.WaitForPodTerminal(ctx, client, created.Namespace, created.Name, wait)
	logPod := created
	if finished != nil {
		logPod = finished
	}

	body, logErr := kubernetes.GetPodLogs(ctx, client, logPod)
	if writeErr := t.writeNodeArtifacts(node, body); writeErr != nil && logErr == nil {
		logErr = writeErr
	}
	if waitErr != nil {
		return fmt.Errorf("%s: %w", node, waitErr)
	}
	if logErr != nil {
		return fmt.Errorf("%s: %w", node, logErr)
	}
	return nil
}

func (t *PerNodeDataTask) debugPod(node string) *corev1.Pod {
	privileged := true
	n := int(t.cfg.Duration.Duration / t.cfg.Interval.Duration)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: debugPodName(node)},
		Spec: corev1.PodSpec{
			NodeName:      node,
			RestartPolicy: corev1.RestartPolicyNever,
			Tolerations:   []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
			Containers: []corev1.Container{{
				Name:            "collect",
				Image:           t.cfg.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"chroot", "/host", "/bin/bash", "-c", collectScript},
				Env: []corev1.EnvVar{
					{Name: "ISOLCPUS", Value: t.cfg.Isolcpus},
					{Name: "TOP_DELAY", Value: fmt.Sprintf("%g", t.cfg.Interval.Seconds())},
					{Name: "TOP_COUNT", Value: strconv.Itoa(n)},
				},
				SecurityContext: &corev1.SecurityContext{Privileged: &privileged},
				VolumeMounts:    []corev1.VolumeMount{{Name: "host", MountPath: "/host"}},
			}},
			Volumes: []corev1.Volume{{
				Name:         "host",
				VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/"}},
			}},
		},
	}
}

func (t *PerNodeDataTask) writeNodeArtifacts(node, body string) error {
	top, mem, cmdline, err := splitCollectLogs(body)
	if err != nil {
		_ = os.WriteFile(filepath.Join(t.artifactsDir, "top-"+node+".out"), []byte(body), 0600)
		return err
	}
	files := map[string]string{
		"top-" + node + ".out":          top,
		"proc_meminfo-" + node + ".txt": mem,
		"proc_cmdline-" + node + ".txt": cmdline,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(t.artifactsDir, name), []byte(content), 0600); err != nil {
			return fmt.Errorf("failed to write %s: %w", name, err)
		}
	}
	return nil
}

func (t *PerNodeDataTask) writeDescribes(ctx context.Context, client *k8s.Clientset) error {
	for _, ns := range t.cfg.WorkloadNamespaces {
		path := filepath.Join(t.artifactsDir, "describe_pods_"+ns+".txt")
		if err := kubernetes.WritePodsYAML(ctx, client, ns, path); err != nil {
			return err
		}
	}
	return kubernetes.WriteNodesYAML(ctx, client, filepath.Join(t.artifactsDir, "describe_nodes.txt"))
}

func splitCollectLogs(body string) (top, mem, cmdline string, err error) {
	memKey := meminfoMarker + "\n"
	cmdKey := cmdlineMarker + "\n"
	i := strings.Index(body, memKey)
	j := strings.Index(body, cmdKey)
	if i < 0 || j < 0 || j < i {
		return "", "", "", fmt.Errorf("pod logs missing %s / %s markers", meminfoMarker, cmdlineMarker)
	}
	return body[:i], body[i+len(memKey) : j], body[j+len(cmdKey):], nil
}

func debugPodName(node string) string {
	var b strings.Builder
	b.WriteString("pnd-")
	prevDash := true
	for _, r := range strings.ToLower(node) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	name := strings.Trim(b.String(), "-")
	if len(name) > 63 {
		name = strings.TrimRight(name[:63], "-")
	}
	return name
}
