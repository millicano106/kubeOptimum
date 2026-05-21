package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PodSummary holds relevant fields from a running Kubernetes Pod.
type PodSummary struct {
	Name      string
	Namespace string
	NodeName  string

	// OwnerDeployment is the name of the owning Deployment (resolved via ReplicaSet).
	// Empty if the Pod is not owned by a Deployment.
	OwnerDeployment string

	Containers []ContainerResources
	Labels     map[string]string
}

// FetchPods lists all running pods in the given namespace (empty = all).
func FetchPods(ctx context.Context, client kubernetes.Interface, namespace string) ([]PodSummary, error) {
	list, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	// Build a ReplicaSet→Deployment map for owner resolution.
	rsOwner, err := buildRSOwnerMap(ctx, client, namespace)
	if err != nil {
		return nil, err
	}

	summaries := make([]PodSummary, 0, len(list.Items))
	for _, p := range list.Items {
		summaries = append(summaries, podToSummary(p, rsOwner))
	}
	return summaries, nil
}

// buildRSOwnerMap returns a map of ReplicaSet name → owning Deployment name.
func buildRSOwnerMap(ctx context.Context, client kubernetes.Interface, namespace string) (map[string]string, error) {
	rsList, err := client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list replicasets: %w", err)
	}
	m := make(map[string]string, len(rsList.Items))
	for _, rs := range rsList.Items {
		for _, ref := range rs.OwnerReferences {
			if ref.Kind == "Deployment" {
				m[rs.Name] = ref.Name
				break
			}
		}
	}
	return m, nil
}

func podToSummary(p corev1.Pod, rsOwner map[string]string) PodSummary {
	var ownerDeploy string
	for _, ref := range p.OwnerReferences {
		if ref.Kind == "ReplicaSet" {
			if deploy, ok := rsOwner[ref.Name]; ok {
				ownerDeploy = deploy
			}
			break
		}
	}

	containers := make([]ContainerResources, 0, len(p.Spec.Containers))
	for _, c := range p.Spec.Containers {
		cr := ContainerResources{Name: c.Name}
		if cpu := c.Resources.Requests.Cpu(); cpu != nil {
			cr.CPURequestMilli = cpu.MilliValue()
		}
		if cpu := c.Resources.Limits.Cpu(); cpu != nil {
			cr.CPULimitMilli = cpu.MilliValue()
		}
		if mem := c.Resources.Requests.Memory(); mem != nil {
			cr.MemRequestBytes = mem.Value()
		}
		if mem := c.Resources.Limits.Memory(); mem != nil {
			cr.MemLimitBytes = mem.Value()
		}
		containers = append(containers, cr)
	}

	labels := p.Labels
	if labels == nil {
		labels = map[string]string{}
	}

	return PodSummary{
		Name:            p.Name,
		Namespace:       p.Namespace,
		NodeName:        p.Spec.NodeName,
		OwnerDeployment: ownerDeploy,
		Containers:      containers,
		Labels:          labels,
	}
}
