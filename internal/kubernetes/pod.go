package kubernetes

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const podPollInterval = 5 * time.Second

// CreatePod creates the pod. Empty namespace becomes "default".
func CreatePod(ctx context.Context, client *kubernetes.Clientset, pod *corev1.Pod) (*corev1.Pod, error) {
	if pod.Namespace == "" {
		pod.Namespace = "default"
	}
	created, err := client.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create pod %s/%s: %w", pod.Namespace, pod.Name, err)
	}
	return created, nil
}

// WaitForPodTerminal polls until Succeeded or Failed, or until timeout / cancel.
func WaitForPodTerminal(ctx context.Context, client *kubernetes.Clientset, namespace, name string, timeout time.Duration) (*corev1.Pod, error) {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(podPollInterval)
	defer ticker.Stop()

	for {
		pod, err := client.CoreV1().Pods(namespace).Get(waitCtx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get pod %s/%s: %w", namespace, name, err)
		}
		switch pod.Status.Phase {
		case corev1.PodSucceeded:
			return pod, nil
		case corev1.PodFailed:
			return pod, fmt.Errorf("pod %s/%s failed", namespace, name)
		}
		select {
		case <-waitCtx.Done():
			return pod, fmt.Errorf("timed out waiting for pod %s/%s: %w", namespace, name, waitCtx.Err())
		case <-ticker.C:
		}
	}
}

// GetPodLogs concatenates logs from pod.Spec.Containers.
func GetPodLogs(ctx context.Context, client *kubernetes.Clientset, pod *corev1.Pod) (string, error) {
	var b strings.Builder
	multi := len(pod.Spec.Containers) > 1
	for _, c := range pod.Spec.Containers {
		req := client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: c.Name})
		rc, err := req.Stream(ctx)
		if err != nil {
			return b.String(), fmt.Errorf("failed to get logs for container %q: %w", c.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return b.String(), err
		}
		if multi {
			fmt.Fprintf(&b, "=== container %s ===\n", c.Name)
		}
		b.Write(data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			b.WriteByte('\n')
		}
	}
	return b.String(), nil
}
