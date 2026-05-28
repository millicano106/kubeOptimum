package checks

import (
	"context"
	"testing"
	"time"

	"github.com/millicano106/kubeOptimum/internal/advisor"
	"github.com/millicano106/kubeOptimum/internal/k8s"
)

func TestSpotOpportunity_FlagsOnDemandStateless(t *testing.T) {
	ac := &advisor.AnalysisContext{
		LookbackWindow: 7 * 24 * time.Hour,
		Thresholds:     defaultThresholds(),
		Deployments: []k8s.DeploymentSummary{
			{Name: "batch-processor", Namespace: "jobs", Replicas: 3},
		},
		Pods: []k8s.PodSummary{
			{
				Name:            "batch-processor-abc",
				Namespace:       "jobs",
				NodeName:        "node-od-1",
				OwnerDeployment: "batch-processor",
			},
		},
		Nodes: []k8s.NodeSummary{
			{
				Name:         "node-od-1",
				InstanceType: "m5.large",
				Labels:       map[string]string{"eks.amazonaws.com/capacityType": "ON_DEMAND"},
			},
		},
		Pricing: map[string]float64{"m5.large": 0.096},
	}

	recs, err := SpotOpportunity{}.Run(context.Background(), ac)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("expected Spot recommendation, got none")
	}
	if recs[0].Category != advisor.CategorySpot {
		t.Errorf("wrong category: %s", recs[0].Category)
	}
	if recs[0].EstimatedMonthlySaving <= 0 {
		t.Error("expected positive estimated saving")
	}
}

func TestSpotOpportunity_AlreadyOnSpotNotFlagged(t *testing.T) {
	ac := &advisor.AnalysisContext{
		LookbackWindow: 7 * 24 * time.Hour,
		Thresholds:     defaultThresholds(),
		Deployments: []k8s.DeploymentSummary{
			{Name: "already-spot", Namespace: "default", Replicas: 1},
		},
		Pods: []k8s.PodSummary{
			{
				Name:            "already-spot-abc",
				Namespace:       "default",
				NodeName:        "spot-node-1",
				OwnerDeployment: "already-spot",
			},
		},
		Nodes: []k8s.NodeSummary{
			{
				Name:   "spot-node-1",
				Labels: map[string]string{"eks.amazonaws.com/capacityType": "SPOT"},
			},
		},
		Pricing: map[string]float64{"m5.large": 0.096},
	}

	recs, err := SpotOpportunity{}.Run(context.Background(), ac)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("expected no Spot recs for deployment already on Spot, got %d", len(recs))
	}
}

func TestSpotOpportunity_NoPricingNoRecommendation(t *testing.T) {
	ac := &advisor.AnalysisContext{
		LookbackWindow: 7 * 24 * time.Hour,
		Thresholds:     defaultThresholds(),
		Deployments: []k8s.DeploymentSummary{
			{Name: "no-pricing", Namespace: "default", Replicas: 2},
		},
		Pods: []k8s.PodSummary{
			{
				Name:            "no-pricing-abc",
				Namespace:       "default",
				NodeName:        "node-1",
				OwnerDeployment: "no-pricing",
			},
		},
		Nodes: []k8s.NodeSummary{
			{Name: "node-1", InstanceType: "m5.large"},
		},
		Pricing: map[string]float64{}, // no pricing data
	}

	recs, err := SpotOpportunity{}.Run(context.Background(), ac)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("expected no recs without pricing data, got %d", len(recs))
	}
}
