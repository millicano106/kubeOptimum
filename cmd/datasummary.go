package cmd

import (
	"fmt"
	"io"

	"github.com/millicano106/kubeOptimum/internal/advisor"
	prommetrics "github.com/millicano106/kubeOptimum/internal/prometheus"
)

// printDataSummary writes a short diagnostic table to w showing how many
// records were collected from each data source. Empty sources are flagged with
// a hint so the user can tell at a glance what isn't configured or reachable.
func printDataSummary(w io.Writer, ac *advisor.AnalysisContext) {
	fmt.Fprintln(w, "--- data sources ---")

	// Kubernetes
	fmt.Fprintf(w, "  nodes:       %d\n", len(ac.Nodes))
	fmt.Fprintf(w, "  deployments: %d\n", len(ac.Deployments))

	podsWithOwner := 0
	for _, p := range ac.Pods {
		if p.OwnerDeployment != "" {
			podsWithOwner++
		}
	}
	fmt.Fprintf(w, "  pods:        %d (%d owned by a Deployment)\n", len(ac.Pods), podsWithOwner)

	if len(ac.Deployments) == 0 {
		fmt.Fprintln(w, "  [!] no deployments found — check --namespace and kubeconfig/context")
	}

	// Prometheus
	podsWithMetrics := make(map[string]struct{})
	for k := range ac.ContainerMetrics {
		podsWithMetrics[k.Namespace+"/"+k.Pod] = struct{}{}
	}
	fmt.Fprintf(w, "  prom containers: %d across %d pods\n", len(ac.ContainerMetrics), len(podsWithMetrics))
	fmt.Fprintf(w, "  prom nodes:      %d\n", len(ac.NodeMetrics))

	if len(ac.ContainerMetrics) == 0 {
		fmt.Fprintln(w, "  [!] no Prometheus container metrics — right-sizing and idle workload checks will produce no results")
		for _, d := range ac.Diagnostics {
			fmt.Fprintf(w, "      %s\n", d)
		}
	} else {
		// Show how many pods have both K8s and Prometheus data.
		matched := countMatchedPods(ac)
		fmt.Fprintf(w, "  pods matched (k8s+prom): %d / %d\n", matched, podsWithOwner)
		if matched == 0 && podsWithOwner > 0 {
			fmt.Fprintln(w, "  [!] pods found in K8s but none matched Prometheus metrics — pod names or namespace labels may differ")
			printSampleKeys(w, ac)
		}
	}

	// Cloud / AWS
	fmt.Fprintf(w, "  nodegroups:  %d\n", len(ac.Nodegroups))
	fmt.Fprintf(w, "  inst types:  %d\n", len(ac.InstanceTypes))
	fmt.Fprintln(w, "--------------------")
}

// countMatchedPods returns how many K8s pods (with an owner deployment) have at
// least one container entry in the Prometheus ContainerMetrics map.
func countMatchedPods(ac *advisor.AnalysisContext) int {
	matched := 0
	for _, pod := range ac.Pods {
		if pod.OwnerDeployment == "" {
			continue
		}
		for _, c := range pod.Containers {
			if _, ok := ac.ContainerMetrics[prommetrics.ContainerKey{
				Namespace: pod.Namespace,
				Pod:       pod.Name,
				Container: c.Name,
			}]; ok {
				matched++
				break
			}
		}
	}
	return matched
}

// printSampleKeys prints one sample pod name from K8s and one from Prometheus to
// help spot label mismatches.
func printSampleKeys(w io.Writer, ac *advisor.AnalysisContext) {
	for _, pod := range ac.Pods {
		if pod.OwnerDeployment != "" {
			fmt.Fprintf(w, "      sample k8s pod:  %s/%s\n", pod.Namespace, pod.Name)
			break
		}
	}
	for k := range ac.ContainerMetrics {
		fmt.Fprintf(w, "      sample prom pod: %s/%s\n", k.Namespace, k.Pod)
		break
	}
}
