package checks

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"

	"github.com/millicano106/kubeOptimum/internal/advisor"
	"github.com/millicano106/kubeOptimum/internal/k8s"
	prommetrics "github.com/millicano106/kubeOptimum/internal/prometheus"
	"github.com/millicano106/kubeOptimum/pkg/units"
)

// RightSizing flags Deployments whose CPU or memory requests are significantly
// larger than their measured p95 usage over the lookback window.
type RightSizing struct{}

func (RightSizing) Name() string        { return "rightsizing" }
func (RightSizing) Description() string { return "CPU and memory request vs p95 actual usage" }

func (RightSizing) Run(ctx context.Context, ac *advisor.AnalysisContext) ([]advisor.Recommendation, error) {
	// Build a map of deployment → per-container p95 usage by aggregating pod metrics.
	type containerUsage struct {
		cpuCores float64
		memBytes float64
	}

	// deployKey: "namespace/deployment/container"
	deployUsage := make(map[string]containerUsage)

	for _, pod := range ac.Pods {
		if pod.OwnerDeployment == "" {
			continue
		}
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
			dk := fmt.Sprintf("%s/%s/%s", pod.Namespace, pod.OwnerDeployment, c.Name)
			existing := deployUsage[dk]
			// Use the max p95 across all pods for conservatism.
			if metrics.CPUP95Cores > existing.cpuCores {
				existing.cpuCores = metrics.CPUP95Cores
			}
			if metrics.MemP95Bytes > existing.memBytes {
				existing.memBytes = metrics.MemP95Bytes
			}
			deployUsage[dk] = existing
		}
	}

	var recs []advisor.Recommendation
	bufferFactor := 1.0 + ac.Thresholds.CPURequestBufferPct/100.0
	memBufferFactor := 1.0 + ac.Thresholds.MemRequestBufferPct/100.0

	for _, deploy := range ac.Deployments {
		for _, c := range deploy.Containers {
			dk := fmt.Sprintf("%s/%s/%s", deploy.Namespace, deploy.Name, c.Name)
			usage, ok := deployUsage[dk]
			if !ok {
				continue
			}

			rec, hasCPURec := cpuRecommendation(deploy, c, usage.cpuCores, bufferFactor, ac)
			if hasCPURec {
				recs = append(recs, rec)
			}

			rec, hasMemRec := memRecommendation(deploy, c, usage.memBytes, memBufferFactor, ac)
			if hasMemRec {
				recs = append(recs, rec)
			}
		}
	}
	return recs, nil
}

func cpuRecommendation(
	deploy k8s.DeploymentSummary,
	c k8s.ContainerResources,
	p95Cores float64,
	bufferFactor float64,
	ac *advisor.AnalysisContext,
) (advisor.Recommendation, bool) {
	if c.CPURequestMilli == 0 {
		return advisor.Recommendation{}, false
	}
	requestCores := float64(c.CPURequestMilli) / 1000.0
	// Flag if request is >2× buffered usage (i.e. more than 2× more than we need).
	threshold := p95Cores * bufferFactor * 2
	if requestCores <= threshold {
		return advisor.Recommendation{}, false
	}

	suggestedMilli := int64(math.Ceil(p95Cores * bufferFactor * 1000))

	saving := estimateCPUSaving(deploy, c.CPURequestMilli, suggestedMilli, ac)
	severity := severityFromRatio(requestCores / (p95Cores * bufferFactor))

	return advisor.Recommendation{
		ID:       recID(advisor.CategoryRightSizing, deploy.Namespace, deploy.Name, c.Name, "cpu"),
		Category: advisor.CategoryRightSizing,
		Severity: severity,
		Resource: advisor.ResourceRef{Kind: "Deployment", Namespace: deploy.Namespace, Name: deploy.Name},
		Summary: fmt.Sprintf("CPU request %s but p95 usage is %s (container: %s)",
			units.FormatCores(requestCores), units.FormatCores(p95Cores), c.Name),
		Detail: fmt.Sprintf(
			"Over the lookback window the p95 CPU for container %q in Deployment %s/%s was %s. "+
				"The current request is %s. Reducing to %s (%s + %.0f%% buffer) would free scheduling capacity.",
			c.Name, deploy.Namespace, deploy.Name,
			units.FormatCores(p95Cores),
			units.FormatCores(requestCores),
			units.FormatCores(float64(suggestedMilli)/1000),
			units.FormatCores(p95Cores),
			ac.Thresholds.CPURequestBufferPct,
		),
		EstimatedMonthlySaving: saving,
		CurrentState: map[string]string{
			"cpu_request":   units.FormatCores(requestCores),
			"p95_cpu_usage": units.FormatCores(p95Cores),
			"container":     c.Name,
		},
		SuggestedAction: map[string]string{
			"cpu_request": units.FormatCores(float64(suggestedMilli) / 1000),
		},
		CheckName: "rightsizing",
	}, true
}

func memRecommendation(
	deploy k8s.DeploymentSummary,
	c k8s.ContainerResources,
	p95Bytes float64,
	bufferFactor float64,
	ac *advisor.AnalysisContext,
) (advisor.Recommendation, bool) {
	if c.MemRequestBytes == 0 {
		return advisor.Recommendation{}, false
	}
	// Flag if request is >2× buffered usage.
	threshold := p95Bytes * bufferFactor * 2
	if float64(c.MemRequestBytes) <= threshold {
		return advisor.Recommendation{}, false
	}

	suggestedBytes := int64(math.Ceil(p95Bytes * bufferFactor))
	severity := severityFromRatio(float64(c.MemRequestBytes) / (p95Bytes * bufferFactor))

	return advisor.Recommendation{
		ID:       recID(advisor.CategoryRightSizing, deploy.Namespace, deploy.Name, c.Name, "mem"),
		Category: advisor.CategoryRightSizing,
		Severity: severity,
		Resource: advisor.ResourceRef{Kind: "Deployment", Namespace: deploy.Namespace, Name: deploy.Name},
		Summary: fmt.Sprintf("Memory request %s but p95 usage is %s (container: %s)",
			units.FormatBytes(c.MemRequestBytes), units.FormatBytes(int64(p95Bytes)), c.Name),
		Detail: fmt.Sprintf(
			"Over the lookback window the p95 memory for container %q in Deployment %s/%s was %s. "+
				"The current request is %s. Reducing to %s (%s + %.0f%% buffer) frees scheduling headroom.",
			c.Name, deploy.Namespace, deploy.Name,
			units.FormatBytes(int64(p95Bytes)),
			units.FormatBytes(c.MemRequestBytes),
			units.FormatBytes(suggestedBytes),
			units.FormatBytes(int64(p95Bytes)),
			ac.Thresholds.MemRequestBufferPct,
		),
		CurrentState: map[string]string{
			"mem_request":   units.FormatBytes(c.MemRequestBytes),
			"p95_mem_usage": units.FormatBytes(int64(p95Bytes)),
			"container":     c.Name,
		},
		SuggestedAction: map[string]string{
			"mem_request": units.FormatBytes(suggestedBytes),
		},
		CheckName: "rightsizing",
	}, true
}

// estimateCPUSaving approximates monthly savings by computing how much of the
// instance cost is attributable to the freed CPU milliCores.
func estimateCPUSaving(deploy k8s.DeploymentSummary, currentMilli, suggestedMilli int64, ac *advisor.AnalysisContext) float64 {
	freedMilli := currentMilli - suggestedMilli
	if freedMilli <= 0 {
		return 0
	}
	// Find the node(s) running this deployment's pods.
	for _, pod := range ac.Pods {
		if pod.OwnerDeployment != deploy.Name || pod.Namespace != deploy.Namespace {
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
			cpuFraction := float64(freedMilli) / float64(node.AllocatableCPUMilli)
			monthlySaving := pricePerHr * cpuFraction * 730 * float64(deploy.Replicas)
			return math.Round(monthlySaving*100) / 100
		}
	}
	return 0
}

func severityFromRatio(ratio float64) advisor.Severity {
	switch {
	case ratio >= 5:
		return advisor.SeverityHigh
	case ratio >= 2:
		return advisor.SeverityMedium
	default:
		return advisor.SeverityLow
	}
}

func recID(category advisor.Category, parts ...string) string {
	h := sha256.New()
	h.Write([]byte(string(category)))
	for _, p := range parts {
		h.Write([]byte(p))
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:8]
}
