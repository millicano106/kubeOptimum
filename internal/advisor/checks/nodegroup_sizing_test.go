package checks

import (
	"context"
	"testing"
	"time"

	"github.com/millicano106/kubeOptimum/internal/advisor"
	"github.com/millicano106/kubeOptimum/internal/cloudprovider"
	"github.com/millicano106/kubeOptimum/internal/k8s"
)

func TestNodegroupSizing_OverProvisionedFlagged(t *testing.T) {
	ac := &advisor.AnalysisContext{
		LookbackWindow: 7 * 24 * time.Hour,
		Thresholds:     defaultThresholds(),
		Nodes: []k8s.NodeSummary{
			{
				Name:                "node-1",
				InstanceType:        "m5.2xlarge",
				NodeGroup:           "general-v2",
				AllocatableCPUMilli: 8000,
				AllocatableMemBytes: 32 * 1024 * 1024 * 1024,
			},
			{
				Name:                "node-2",
				InstanceType:        "m5.2xlarge",
				NodeGroup:           "general-v2",
				AllocatableCPUMilli: 8000,
				AllocatableMemBytes: 32 * 1024 * 1024 * 1024,
			},
		},
		Pods: []k8s.PodSummary{
			{
				Name:     "app-pod-1",
				NodeName: "node-1",
				Containers: []k8s.ContainerResources{
					{CPURequestMilli: 200, MemRequestBytes: 256 * 1024 * 1024},
				},
			},
		},
		Nodegroups: []cloudprovider.NodegroupSummary{
			{Name: "general-v2", InstanceType: "m5.2xlarge", DesiredSize: 2},
		},
		Pricing: map[string]float64{"m5.2xlarge": 0.384},
	}

	recs, err := NodegroupSizing{}.Run(context.Background(), ac)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("expected nodegroup recommendation, got none")
	}
	if recs[0].Category != advisor.CategoryNodegroup {
		t.Errorf("wrong category: %s", recs[0].Category)
	}
	if recs[0].EstimatedMonthlySaving <= 0 {
		t.Error("expected positive estimated saving")
	}
}

func TestNodegroupSizing_WellUtilisedNotFlagged(t *testing.T) {
	ac := &advisor.AnalysisContext{
		LookbackWindow: 7 * 24 * time.Hour,
		Thresholds:     defaultThresholds(),
		Nodes: []k8s.NodeSummary{
			{
				Name:                "node-1",
				InstanceType:        "m5.large",
				NodeGroup:           "tight-group",
				AllocatableCPUMilli: 2000,
				AllocatableMemBytes: 8 * 1024 * 1024 * 1024,
			},
		},
		Pods: []k8s.PodSummary{
			{
				Name:     "app-1",
				NodeName: "node-1",
				Containers: []k8s.ContainerResources{
					{CPURequestMilli: 1600, MemRequestBytes: 6 * 1024 * 1024 * 1024},
				},
			},
		},
		Nodegroups: []cloudprovider.NodegroupSummary{
			{Name: "tight-group", InstanceType: "m5.large", DesiredSize: 1},
		},
	}

	recs, err := NodegroupSizing{}.Run(context.Background(), ac)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("expected no recs for well-utilised nodegroup, got %d", len(recs))
	}
}
