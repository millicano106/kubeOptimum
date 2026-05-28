package checks

import (
	"context"
	"testing"
	"time"

	"github.com/millicano106/kubeOptimum/internal/advisor"
	"github.com/millicano106/kubeOptimum/internal/cloudprovider"
	"github.com/millicano106/kubeOptimum/internal/k8s"
)

func TestInstanceType_SuggestsSmallerInFamily(t *testing.T) {
	ac := &advisor.AnalysisContext{
		LookbackWindow: 7 * 24 * time.Hour,
		Thresholds:     defaultThresholds(),
		Nodes: []k8s.NodeSummary{
			{
				Name:                "node-1",
				InstanceType:        "m5.2xlarge",
				NodeGroup:           "general",
				AllocatableCPUMilli: 8000,
				AllocatableMemBytes: 32 * 1024 * 1024 * 1024,
			},
		},
		Pods: []k8s.PodSummary{
			{
				Name:     "app-1",
				NodeName: "node-1",
				Containers: []k8s.ContainerResources{
					// Pod requests only 1500m CPU and 6 GiB — fits on m5.xlarge
					{CPURequestMilli: 1500, MemRequestBytes: 6 * 1024 * 1024 * 1024},
				},
			},
		},
		Nodegroups: []cloudprovider.NodegroupSummary{
			{Name: "general", InstanceType: "m5.2xlarge", DesiredSize: 1},
		},
		InstanceTypes: map[string]cloudprovider.InstanceTypeDetail{
			"m5.2xlarge": {Name: "m5.2xlarge", VCPUs: 8, MemoryMiB: 32 * 1024},
			"m5.xlarge":  {Name: "m5.xlarge", VCPUs: 4, MemoryMiB: 16 * 1024},
			"m5.large":   {Name: "m5.large", VCPUs: 2, MemoryMiB: 8 * 1024},
		},
		Pricing: map[string]float64{
			"m5.2xlarge": 0.384,
			"m5.xlarge":  0.192,
			"m5.large":   0.096,
		},
	}

	recs, err := InstanceType{}.Run(context.Background(), ac)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("expected instance type recommendation, got none")
	}
	if recs[0].Category != advisor.CategoryInstanceType {
		t.Errorf("wrong category: %s", recs[0].Category)
	}
	suggested := recs[0].SuggestedAction["instance_type"]
	if suggested == "m5.2xlarge" {
		t.Error("suggested same instance type as current")
	}
	if recs[0].EstimatedMonthlySaving <= 0 {
		t.Error("expected positive estimated saving")
	}
}

func TestInstanceType_NoSmallerFitAvailable(t *testing.T) {
	ac := &advisor.AnalysisContext{
		LookbackWindow: 7 * 24 * time.Hour,
		Thresholds:     defaultThresholds(),
		Nodes: []k8s.NodeSummary{
			{
				Name:                "node-1",
				InstanceType:        "m5.large",
				NodeGroup:           "tight",
				AllocatableCPUMilli: 2000,
				AllocatableMemBytes: 8 * 1024 * 1024 * 1024,
			},
		},
		Pods: []k8s.PodSummary{
			{
				Name:     "heavy-pod",
				NodeName: "node-1",
				Containers: []k8s.ContainerResources{
					{CPURequestMilli: 1800, MemRequestBytes: 7 * 1024 * 1024 * 1024},
				},
			},
		},
		Nodegroups: []cloudprovider.NodegroupSummary{
			{Name: "tight", InstanceType: "m5.large", DesiredSize: 1},
		},
		InstanceTypes: map[string]cloudprovider.InstanceTypeDetail{
			"m5.large": {Name: "m5.large", VCPUs: 2, MemoryMiB: 8 * 1024},
			// no smaller m5 family types
		},
		Pricing: map[string]float64{"m5.large": 0.096},
	}

	recs, err := InstanceType{}.Run(context.Background(), ac)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("expected no recs when no smaller instance fits, got %d", len(recs))
	}
}
