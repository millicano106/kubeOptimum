package checks

import (
	"context"
	"fmt"

	"github.com/millicano106/kubeOptimum/internal/advisor"
	prommetrics "github.com/millicano106/kubeOptimum/internal/prometheus"
	"github.com/millicano106/kubeOptimum/pkg/units"
)

// IdleWorkloads flags Deployments whose peak CPU usage never exceeded the
// configured idle threshold over the full lookback window.
type IdleWorkloads struct{}

func (IdleWorkloads) Name() string { return "idle_workloads" }
func (IdleWorkloads) Description() string {
	return "Deployments with near-zero CPU usage over the lookback window"
}

func (IdleWorkloads) Run(ctx context.Context, ac *advisor.AnalysisContext) ([]advisor.Recommendation, error) {
	// Aggregate max CPU per deployment across all its pods.
	deployMaxCPU := make(map[string]float64) // "namespace/deploy" → max cores
	deployReplicas := make(map[string]int32)

	for _, pod := range ac.Pods {
		if pod.OwnerDeployment == "" {
			continue
		}
		deployKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.OwnerDeployment)
		for _, c := range pod.Containers {
			key := prommetrics.ContainerKey{
				Namespace: pod.Namespace,
				Pod:       pod.Name,
				Container: c.Name,
			}
			metrics, ok := ac.ContainerMetrics[key]
			if !ok {
				continue
			}
			if metrics.CPUP95Cores > deployMaxCPU[deployKey] {
				deployMaxCPU[deployKey] = metrics.CPUP95Cores
			}
		}
	}

	for _, deploy := range ac.Deployments {
		deployKey := fmt.Sprintf("%s/%s", deploy.Namespace, deploy.Name)
		deployReplicas[deployKey] = deploy.Replicas
	}

	idleThresholdCores := ac.Thresholds.IdleCPUThresholdMilli / 1000.0

	var recs []advisor.Recommendation
	for deployKey, maxCPU := range deployMaxCPU {
		if maxCPU > idleThresholdCores {
			continue
		}
		replicas := deployReplicas[deployKey]

		var ns, name string
		fmt.Sscanf(deployKey, "%s", &ns) // not ideal but see below
		// Split manually to avoid importing strings.
		for i, ch := range deployKey {
			if ch == '/' {
				ns = deployKey[:i]
				name = deployKey[i+1:]
				break
			}
		}
		if name == "" {
			continue
		}

		saving := estimateIdleSaving(ns, name, replicas, ac)

		recs = append(recs, advisor.Recommendation{
			ID:       recID(advisor.CategoryIdleWorkload, ns, name),
			Category: advisor.CategoryIdleWorkload,
			Severity: advisor.SeverityHigh,
			Resource: advisor.ResourceRef{Kind: "Deployment", Namespace: ns, Name: name},
			Summary: fmt.Sprintf("Max CPU %s over lookback window — workload appears idle",
				units.FormatCores(maxCPU)),
			Detail: fmt.Sprintf(
				"Deployment %s/%s had a maximum p95 CPU of %s across all pods over the lookback window, "+
					"below the idle threshold of %s. Consider scaling to zero replicas or removing this Deployment.",
				ns, name,
				units.FormatCores(maxCPU),
				units.FormatCores(idleThresholdCores),
			),
			EstimatedMonthlySaving: saving,
			CurrentState: map[string]string{
				"max_p95_cpu": units.FormatCores(maxCPU),
				"replicas":    fmt.Sprintf("%d", replicas),
			},
			SuggestedAction: map[string]string{
				"replicas": "0",
			},
			CheckName: "idle_workloads",
		})
	}
	return recs, nil
}

func estimateIdleSaving(ns, deployName string, replicas int32, ac *advisor.AnalysisContext) float64 {
	for _, pod := range ac.Pods {
		if pod.OwnerDeployment != deployName || pod.Namespace != ns {
			continue
		}
		for _, node := range ac.Nodes {
			if node.Name != pod.NodeName {
				continue
			}
			pricePerHr, ok := ac.Pricing[node.InstanceType]
			if !ok || node.AllocatableCPUMilli == 0 {
				continue
			}
			// Rough estimate: assume ~10 pods per node to get per-pod cost share.
			const assumedPodsPerNode = 10
			nodeHourlyShared := pricePerHr / assumedPodsPerNode
			return nodeHourlyShared * 730 * float64(replicas)
		}
	}
	return 0
}
