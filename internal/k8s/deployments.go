package k8s

import (
	"context"
	"fmt"
	"log/slog"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ContainerResources holds the resource requests and limits for one container.
type ContainerResources struct {
	Name string

	CPURequestMilli int64 // millicores; 0 = unset
	CPULimitMilli   int64
	MemRequestBytes int64 // bytes; 0 = unset
	MemLimitBytes   int64
}

// DeploymentSummary holds the fields we care about from a Kubernetes Deployment.
type DeploymentSummary struct {
	Name       string
	Namespace  string
	Replicas   int32
	Containers []ContainerResources
	Labels     map[string]string
}

// FetchDeployments lists all deployments in the given namespace (empty = all).
func FetchDeployments(ctx context.Context, client kubernetes.Interface, namespace string) ([]DeploymentSummary, error) {
	slog.Debug("fetching deployments", "namespace", namespace)
	list, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}

	summaries := make([]DeploymentSummary, 0, len(list.Items))
	for _, d := range list.Items {
		summaries = append(summaries, deploymentToSummary(d))
	}
	slog.Debug("fetched deployments", "count", len(summaries))
	return summaries, nil
}

func deploymentToSummary(d appsv1.Deployment) DeploymentSummary {
	var replicas int32 = 1
	if d.Spec.Replicas != nil {
		replicas = *d.Spec.Replicas
	}

	containers := make([]ContainerResources, 0, len(d.Spec.Template.Spec.Containers))
	for _, c := range d.Spec.Template.Spec.Containers {
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

	labels := d.Labels
	if labels == nil {
		labels = map[string]string{}
	}

	return DeploymentSummary{
		Name:       d.Name,
		Namespace:  d.Namespace,
		Replicas:   replicas,
		Containers: containers,
		Labels:     labels,
	}
}
