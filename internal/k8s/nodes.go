package k8s

import (
	"context"
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NodeSummary holds the fields we care about from a Kubernetes Node.
type NodeSummary struct {
	Name         string
	InstanceType string // from node.labels["node.kubernetes.io/instance-type"]
	Region       string // from node.labels["topology.kubernetes.io/region"]
	Zone         string // from node.labels["topology.kubernetes.io/zone"]
	NodeGroup    string // from node.labels["eks.amazonaws.com/nodegroup"]

	AllocatableCPUMilli int64 // millicores
	AllocatableMemBytes int64 // bytes

	Labels map[string]string
}

// FetchNodes lists all nodes and returns summarised information.
func FetchNodes(ctx context.Context, client kubernetes.Interface) ([]NodeSummary, error) {
	slog.Debug("fetching nodes")
	list, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	summaries := make([]NodeSummary, 0, len(list.Items))
	for _, n := range list.Items {
		summaries = append(summaries, nodeToSummary(n))
	}
	slog.Debug("fetched nodes", "count", len(summaries))
	return summaries, nil
}

func nodeToSummary(n corev1.Node) NodeSummary {
	labels := n.Labels
	if labels == nil {
		labels = map[string]string{}
	}

	cpuQty := n.Status.Allocatable[corev1.ResourceCPU]
	memQty := n.Status.Allocatable[corev1.ResourceMemory]

	return NodeSummary{
		Name:                n.Name,
		InstanceType:        labels["node.kubernetes.io/instance-type"],
		Region:              labels["topology.kubernetes.io/region"],
		Zone:                labels["topology.kubernetes.io/zone"],
		NodeGroup:           labels["eks.amazonaws.com/nodegroup"],
		AllocatableCPUMilli: cpuQty.MilliValue(),
		AllocatableMemBytes: memQty.Value(),
		Labels:              labels,
	}
}
