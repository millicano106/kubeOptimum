package checks

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/millicano106/kubeOptimum/internal/advisor"
	awsprovider "github.com/millicano106/kubeOptimum/internal/cloudprovider/aws"
)

// InstanceType flags nodes running on instance types where a smaller type in
// the same family could satisfy the workload's actual resource requests.
type InstanceType struct{}

func (InstanceType) Name() string        { return "instance_type" }
func (InstanceType) Description() string { return "Suggest cheaper EC2 instance type in same family" }

func (InstanceType) Run(ctx context.Context, ac *advisor.AnalysisContext) ([]advisor.Recommendation, error) {
	if len(ac.InstanceTypes) == 0 {
		return nil, nil
	}

	// Aggregate actual CPU/memory requests per nodegroup.
	nodeToGroup := make(map[string]string)
	for _, n := range ac.Nodes {
		if n.NodeGroup != "" {
			nodeToGroup[n.Name] = n.NodeGroup
		}
	}

	type ngState struct {
		instanceType    string
		maxCPUMilli     int64 // highest single-node request sum
		maxMemBytes     int64
		allocCPUMilli   int64 // per-node allocatable (first node)
		allocMemBytes   int64
	}
	ngMap := make(map[string]*ngState)

	for _, n := range ac.Nodes {
		ng := nodeToGroup[n.Name]
		if ng == "" {
			continue
		}
		if ngMap[ng] == nil {
			ngMap[ng] = &ngState{
				instanceType:  n.InstanceType,
				allocCPUMilli: n.AllocatableCPUMilli,
				allocMemBytes: n.AllocatableMemBytes,
			}
		}
	}

	// Per-node pod request totals.
	nodeCPU := make(map[string]int64)
	nodeMem := make(map[string]int64)
	for _, pod := range ac.Pods {
		for _, c := range pod.Containers {
			nodeCPU[pod.NodeName] += c.CPURequestMilli
			nodeMem[pod.NodeName] += c.MemRequestBytes
		}
	}
	for nodeName, ng := range nodeToGroup {
		if ngMap[ng] == nil {
			continue
		}
		if nodeCPU[nodeName] > ngMap[ng].maxCPUMilli {
			ngMap[ng].maxCPUMilli = nodeCPU[nodeName]
		}
		if nodeMem[nodeName] > ngMap[ng].maxMemBytes {
			ngMap[ng].maxMemBytes = nodeMem[nodeName]
		}
	}

	var recs []advisor.Recommendation
	for ngName, state := range ngMap {
		current, ok := ac.InstanceTypes[state.instanceType]
		if !ok {
			continue
		}

		candidates := awsprovider.SameFamily(state.instanceType, ac.InstanceTypes, current)
		if len(candidates) == 0 {
			continue
		}

		// Sort by vCPUs descending so we suggest the largest that still fits.
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].VCPUs > candidates[j].VCPUs
		})

		// Find the best candidate: smaller vCPUs but still able to fit the max node load.
		for _, candidate := range candidates {
			candidateAllocMilli := int64(candidate.VCPUs) * 1000
			candidateAllocMem := candidate.MemoryMiB * 1024 * 1024

			if candidateAllocMilli < state.maxCPUMilli || candidateAllocMem < state.maxMemBytes {
				continue
			}

			saving := instanceTypeSaving(state.instanceType, candidate.Name, ac)
			if saving <= 0 {
				continue
			}

			recs = append(recs, advisor.Recommendation{
				ID:       recID(advisor.CategoryInstanceType, ngName),
				Category: advisor.CategoryInstanceType,
				Severity: advisor.SeverityMedium,
				Resource: advisor.ResourceRef{Kind: "Nodegroup", Name: ngName},
				Summary: fmt.Sprintf("Switch %s → %s (same family, lower cost)",
					state.instanceType, candidate.Name),
				Detail: fmt.Sprintf(
					"Nodegroup %q uses %s (%d vCPU, %d MiB). "+
						"The highest per-node CPU request is %dm and memory is %s. "+
						"%s (%d vCPU, %d MiB) would satisfy these requirements at lower cost.",
					ngName,
					state.instanceType, current.VCPUs, current.MemoryMiB,
					state.maxCPUMilli,
					fmt.Sprintf("%d MiB", state.maxMemBytes/(1024*1024)),
					candidate.Name, candidate.VCPUs, candidate.MemoryMiB,
				),
				EstimatedMonthlySaving: saving,
				CurrentState: map[string]string{
					"instance_type": state.instanceType,
					"vcpus":         fmt.Sprintf("%d", current.VCPUs),
					"memory_mib":    fmt.Sprintf("%d", current.MemoryMiB),
				},
				SuggestedAction: map[string]string{
					"instance_type": candidate.Name,
				},
				CheckName: "instance_type",
			})
			break // recommend only the best candidate per nodegroup
		}
	}
	return recs, nil
}

func instanceTypeSaving(current, suggested string, ac *advisor.AnalysisContext) float64 {
	currentPrice, ok1 := ac.Pricing[current]
	suggestedPrice, ok2 := ac.Pricing[suggested]
	if !ok1 || !ok2 {
		return 0
	}
	diff := currentPrice - suggestedPrice
	if diff <= 0 {
		return 0
	}
	// Count nodes using this instance type.
	nodeCount := 0
	for _, n := range ac.Nodes {
		if n.InstanceType == current {
			nodeCount++
		}
	}
	return math.Round(diff*float64(nodeCount)*730*100) / 100
}
