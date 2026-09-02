package kubernetes

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8syaml "sigs.k8s.io/yaml"
)

// ListNodeNames returns metadata.name for every node. Fails if there are none.
func ListNodeNames(ctx context.Context, client *kubernetes.Clientset) ([]string, error) {
	list, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}
	if len(list.Items) == 0 {
		return nil, fmt.Errorf("no nodes found")
	}
	names := make([]string, 0, len(list.Items))
	for i := range list.Items {
		names = append(names, list.Items[i].Name)
	}
	return names, nil
}

// WritePodsYAML writes a v1 PodList for namespace to path. An empty list is OK.
func WritePodsYAML(ctx context.Context, client *kubernetes.Clientset, namespace, path string) error {
	list, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods in %s: %w", namespace, err)
	}
	list.APIVersion = "v1"
	list.Kind = "PodList"
	return writeYAML(path, list)
}

// WriteNodesYAML writes a v1 NodeList to path.
func WriteNodesYAML(ctx context.Context, client *kubernetes.Clientset, path string) error {
	list, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}
	list.APIVersion = "v1"
	list.Kind = "NodeList"
	return writeYAML(path, list)
}

func writeYAML(path string, obj any) error {
	body, err := k8syaml.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", path, err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}
