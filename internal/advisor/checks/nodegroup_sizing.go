package checks

import (
	"context"
	"fmt"
	"math"

	"github.com/millicano106/kubeOptimum/internal/advisor"
	"github.com/millicano106/kubeOptimum/internal/cloudprovider"
	"github.com/millicano106/kubeOptimum/pkg/units"
)

// headroomThreshold is the fraction of allocatable capacity that must be free
// before we flag a nodegroup as over-provisioned.
const headroomThreshold = 0.40

type ngUsage struct {
	cpuRequestMilli int64
	memRequestBytes int64
	allocCPUMilli   int64
	allocMemBytes   int64
	nodeCount       int32
	instanceType    string
}

// NodegroupSizing flags EKS nodegroups where total pod CPU/memory requests
// leave more than 40% of allocatable capacity unused.
type NodegroupSizing struct{}

func (NodegroupSizing) Name() string        { return "nodegroup_sizing" }
func (NodegroupSizing) Description() string { return "EKS nodegroup headroom vs pod request sum" }

func (NodegroupSizing) Run(ctx context.Context, ac *advisor.AnalysisContext) ([]advisor.Recommendation, error) {
	nodeToGroup := make(map[string]string, len(ac.Nodes))
	for _, n := range ac.Nodes {
		if n.NodeGroup != "" {
			nodeToGroup[n.Name] = n.NodeGroup
		}
	}

	groups := make(map[string]*ngUsage)
	for _, n := range ac.Nodes {
		ng := nodeToGroup[n.Name]
		if ng == "" {
			continue
		}
		if groups[ng] == nil {
			groups[ng] = &ngUsage{instanceType: n.InstanceType}
		}
		groups[ng].allocCPUMilli += n.AllocatableCPUMilli
		groups[ng].allocMemBytes += n.AllocatableMemBytes
		groups[ng].nodeCount++
	}

	for _, pod := range ac.Pods {
		ng := nodeToGroup[pod.NodeName]
		if ng == "" || groups[ng] == nil {
			continue
		}
		for _, c := range pod.Containers {
			groups[ng].cpuRequestMilli += c.CPURequestMilli
			groups[ng].memRequestBytes += c.MemRequestBytes
		}
	}

	var recs []advisor.Recommendation
	for ngName, usage := range groups {
		if usage.allocCPUMilli == 0 {
			continue
		}

		cpuHeadroom := 1.0 - float64(usage.cpuRequestMilli)/float64(usage.allocCPUMilli)
		memHeadroom := 0.0
		if usage.allocMemBytes > 0 {
			memHeadroom = 1.0 - float64(usage.memRequestBytes)/float64(usage.allocMemBytes)
		}

		if cpuHeadroom < headroomThreshold && memHeadroom < headroomThreshold {
			continue
		}

		var ngSummary cloudprovider.NodegroupSummary
		for _, s := range ac.Nodegroups {
			if s.Name == ngName {
				ngSummary = s
				break
			}
		}

		saving := ngSavingEstimate(usage, ac)
		suggested := ngSuggestDesired(usage)

		recs = append(recs, advisor.Recommendation{
			ID:       recID(advisor.CategoryNodegroup, ngName),
			Category: advisor.CategoryNodegroup,
			Severity: severityFromHeadroom(cpuHeadroom, memHeadroom),
			Resource: advisor.ResourceRef{Kind: "Nodegroup", Name: ngName},
			Summary: fmt.Sprintf("%.0f%% CPU headroom — nodegroup may be over-provisioned (%d nodes, %s)",
				cpuHeadroom*100, usage.nodeCount, usage.instanceType),
			Detail: fmt.Sprintf(
				"Nodegroup %q has %d nodes of type %s. "+
					"Pod CPU requests total %s against %s allocatable (%.0f%% headroom). "+
					"Memory requests total %s against %s allocatable (%.0f%% headroom). "+
					"Consider reducing desired count or switching to a smaller instance type.",
				ngName, usage.nodeCount, usage.instanceType,
				units.FormatCores(float64(usage.cpuRequestMilli)/1000),
				units.FormatCores(float64(usage.allocCPUMilli)/1000),
				cpuHeadroom*100,
				units.FormatBytes(usage.memRequestBytes),
				units.FormatBytes(usage.allocMemBytes),
				memHeadroom*100,
			),
			EstimatedMonthlySaving: saving,
			CurrentState: map[string]string{
				"node_count":       fmt.Sprintf("%d", usage.nodeCount),
				"instance_type":    usage.instanceType,
				"cpu_headroom_pct": fmt.Sprintf("%.0f%%", cpuHeadroom*100),
				"mem_headroom_pct": fmt.Sprintf("%.0f%%", memHeadroom*100),
				"desired_size":     fmt.Sprintf("%d", ngSummary.DesiredSize),
			},
			SuggestedAction: map[string]string{
				"desired_size": fmt.Sprintf("%d", suggested),
			},
			CheckName: "nodegroup_sizing",
		})
	}
	return recs, nil
}

func severityFromHeadroom(cpu, mem float64) advisor.Severity {
	max := cpu
	if mem > max {
		max = mem
	}
	switch {
	case max >= 0.65:
		return advisor.SeverityHigh
	case max >= 0.45:
		return advisor.SeverityMedium
	default:
		return advisor.SeverityLow
	}
}

func ngSuggestDesired(usage *ngUsage) int32 {
	const targetUtil = 0.80
	perNodeCPU := float64(usage.allocCPUMilli) / float64(usage.nodeCount)
	if perNodeCPU == 0 {
		return usage.nodeCount
	}
	needed := math.Ceil(float64(usage.cpuRequestMilli) / (perNodeCPU * targetUtil))
	if needed < 1 {
		needed = 1
	}
	return int32(needed)
}

func ngSavingEstimate(usage *ngUsage, ac *advisor.AnalysisContext) float64 {
	pricePerHr, ok := ac.Pricing[usage.instanceType]
	if !ok || usage.nodeCount == 0 {
		return 0
	}
	removedNodes := usage.nodeCount - ngSuggestDesired(usage)
	if removedNodes <= 0 {
		return 0
	}
	return math.Round(pricePerHr*float64(removedNodes)*730*100) / 100
}
