package checks

import (
	"context"
	"testing"
	"time"

	"github.com/millicano106/kubeOptimum/internal/advisor"
	"github.com/millicano106/kubeOptimum/internal/k8s"
	prommetrics "github.com/millicano106/kubeOptimum/internal/prometheus"
)

func TestIdleWorkloads_FlagsIdleDeployment(t *testing.T) {
	ac := &advisor.AnalysisContext{
		LookbackWindow: 7 * 24 * time.Hour,
		Thresholds:     defaultThresholds(), // IdleCPUThresholdMilli = 5
		Deployments: []k8s.DeploymentSummary{
			{Name: "legacy-reporter", Namespace: "staging", Replicas: 2},
		},
		Pods: []k8s.PodSummary{
			{
				Name:            "legacy-reporter-abc",
				Namespace:       "staging",
				NodeName:        "node-1",
				OwnerDeployment: "legacy-reporter",
				Containers:      []k8s.ContainerResources{{Name: "app"}},
			},
		},
		ContainerMetrics: map[prommetrics.ContainerKey]prommetrics.ContainerMetrics{
			{Namespace: "staging", Pod: "legacy-reporter-abc", Container: "app"}: {
				CPUP95Cores: 0.002, // 2m — below 5m idle threshold
			},
		},
	}

	recs, err := IdleWorkloads{}.Run(context.Background(), ac)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("expected idle recommendation, got none")
	}
	if recs[0].Category != advisor.CategoryIdleWorkload {
		t.Errorf("wrong category: %s", recs[0].Category)
	}
	if recs[0].Resource.Name != "legacy-reporter" {
		t.Errorf("wrong resource name: %s", recs[0].Resource.Name)
	}
}

func TestIdleWorkloads_ActiveDeploymentNotFlagged(t *testing.T) {
	ac := &advisor.AnalysisContext{
		LookbackWindow: 7 * 24 * time.Hour,
		Thresholds:     defaultThresholds(),
		Deployments: []k8s.DeploymentSummary{
			{Name: "active-api", Namespace: "production", Replicas: 3},
		},
		Pods: []k8s.PodSummary{
			{
				Name:            "active-api-xyz",
				Namespace:       "production",
				OwnerDeployment: "active-api",
				Containers:      []k8s.ContainerResources{{Name: "app"}},
			},
		},
		ContainerMetrics: map[prommetrics.ContainerKey]prommetrics.ContainerMetrics{
			{Namespace: "production", Pod: "active-api-xyz", Container: "app"}: {
				CPUP95Cores: 0.250, // 250m — well above threshold
			},
		},
	}

	recs, err := IdleWorkloads{}.Run(context.Background(), ac)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("expected no recs for active deployment, got %d", len(recs))
	}
}
