package checks

import (
	"context"
	"testing"
	"time"

	"github.com/millicano106/kubeOptimum/internal/advisor"
	"github.com/millicano106/kubeOptimum/internal/k8s"
	prommetrics "github.com/millicano106/kubeOptimum/internal/prometheus"
)

func defaultThresholds() advisor.Thresholds {
	return advisor.Thresholds{
		CPURequestBufferPct:   20,
		MemRequestBufferPct:   20,
		IdleCPUThresholdMilli: 5,
	}
}

func TestRightSizing_CPUOverProvisioned(t *testing.T) {
	ac := &advisor.AnalysisContext{
		LookbackWindow: 7 * 24 * time.Hour,
		Thresholds:     defaultThresholds(),
		Deployments: []k8s.DeploymentSummary{
			{
				Name:      "payments-api",
				Namespace: "default",
				Replicas:  2,
				Containers: []k8s.ContainerResources{
					{Name: "app", CPURequestMilli: 500, MemRequestBytes: 256 * 1024 * 1024},
				},
			},
		},
		Pods: []k8s.PodSummary{
			{
				Name:            "payments-api-abc-123",
				Namespace:       "default",
				NodeName:        "node-1",
				OwnerDeployment: "payments-api",
				Containers:      []k8s.ContainerResources{{Name: "app"}},
			},
		},
		Nodes: []k8s.NodeSummary{
			{Name: "node-1", InstanceType: "m5.large", AllocatableCPUMilli: 2000, AllocatableMemBytes: 7 * 1024 * 1024 * 1024},
		},
		ContainerMetrics: map[prommetrics.ContainerKey]prommetrics.ContainerMetrics{
			{Namespace: "default", Pod: "payments-api-abc-123", Container: "app"}: {
				CPUP95Cores: 0.012, // 12m — request is 500m, way over-provisioned
				MemP95Bytes: 100 * 1024 * 1024,
			},
		},
		Pricing: map[string]float64{"m5.large": 0.096},
	}

	recs, err := RightSizing{}.Run(context.Background(), ac)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("expected at least one recommendation, got none")
	}

	// Should flag CPU over-provisioning.
	found := false
	for _, r := range recs {
		if r.Category == advisor.CategoryRightSizing && r.CurrentState["container"] == "app" {
			if r.CurrentState["cpu_request"] == "500m" {
				found = true
				if r.Severity != advisor.SeverityHigh {
					t.Errorf("expected HIGH severity, got %s", r.Severity)
				}
			}
		}
	}
	if !found {
		t.Error("expected a right-sizing recommendation for CPU")
	}
}

func TestRightSizing_NoRecommendationWhenWellSized(t *testing.T) {
	ac := &advisor.AnalysisContext{
		LookbackWindow: 7 * 24 * time.Hour,
		Thresholds:     defaultThresholds(),
		Deployments: []k8s.DeploymentSummary{
			{
				Name:      "well-sized",
				Namespace: "default",
				Replicas:  1,
				Containers: []k8s.ContainerResources{
					{Name: "app", CPURequestMilli: 100, MemRequestBytes: 128 * 1024 * 1024},
				},
			},
		},
		Pods: []k8s.PodSummary{
			{
				Name:            "well-sized-abc",
				Namespace:       "default",
				NodeName:        "node-1",
				OwnerDeployment: "well-sized",
				Containers:      []k8s.ContainerResources{{Name: "app"}},
			},
		},
		Nodes: []k8s.NodeSummary{
			{Name: "node-1", InstanceType: "m5.large", AllocatableCPUMilli: 2000},
		},
		ContainerMetrics: map[prommetrics.ContainerKey]prommetrics.ContainerMetrics{
			{Namespace: "default", Pod: "well-sized-abc", Container: "app"}: {
				CPUP95Cores: 0.080, // 80m vs 100m request — within 2× buffer
				MemP95Bytes: 100 * 1024 * 1024,
			},
		},
	}

	recs, err := RightSizing{}.Run(context.Background(), ac)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("expected no recommendations for well-sized deployment, got %d", len(recs))
	}
}

func TestRightSizing_NoMetricsNoRecommendation(t *testing.T) {
	ac := &advisor.AnalysisContext{
		LookbackWindow: 7 * 24 * time.Hour,
		Thresholds:     defaultThresholds(),
		Deployments: []k8s.DeploymentSummary{
			{
				Name:      "no-metrics",
				Namespace: "default",
				Replicas:  1,
				Containers: []k8s.ContainerResources{
					{Name: "app", CPURequestMilli: 500},
				},
			},
		},
		Pods: []k8s.PodSummary{
			{Name: "no-metrics-abc", Namespace: "default", OwnerDeployment: "no-metrics"},
		},
		ContainerMetrics: map[prommetrics.ContainerKey]prommetrics.ContainerMetrics{},
	}

	recs, err := RightSizing{}.Run(context.Background(), ac)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("expected no recs when no metrics available, got %d", len(recs))
	}
}
