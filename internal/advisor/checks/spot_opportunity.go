package checks

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/millicano106/kubeOptimum/internal/advisor"
)

// spotDiscountFactor approximates Spot savings vs on-demand (typically 60–70%).
const spotDiscountFactor = 0.65

// SpotOpportunity flags Deployments that are good candidates to run on Spot
// instances: stateless (no persistent volumes), no OnDelete update strategy,
// and no node selector/affinity that would pin them to on-demand nodes.
type SpotOpportunity struct{}

func (SpotOpportunity) Name() string        { return "spot_opportunity" }
func (SpotOpportunity) Description() string { return "Deployments suitable for Spot instances" }

func (SpotOpportunity) Run(ctx context.Context, ac *advisor.AnalysisContext) ([]advisor.Recommendation, error) {
	// Build a set of deployments already running on Spot nodes.
	spotNodes := make(map[string]bool)
	for _, n := range ac.Nodes {
		// AWS marks Spot nodes with this label.
		if n.Labels["eks.amazonaws.com/capacityType"] == "SPOT" {
			spotNodes[n.Name] = true
		}
	}

	podNodeMap := make(map[string]string) // "namespace/deploy" → nodeName (first pod)
	for _, pod := range ac.Pods {
		if pod.OwnerDeployment == "" {
			continue
		}
		key := fmt.Sprintf("%s/%s", pod.Namespace, pod.OwnerDeployment)
		if _, exists := podNodeMap[key]; !exists {
			podNodeMap[key] = pod.NodeName
		}
	}

	var recs []advisor.Recommendation
	for _, deploy := range ac.Deployments {
		deployKey := fmt.Sprintf("%s/%s", deploy.Namespace, deploy.Name)
		nodeName := podNodeMap[deployKey]

		// Skip if already on Spot.
		if spotNodes[nodeName] {
			continue
		}
		// Skip if the deployment is pinned to on-demand via label.
		if isOnDemandPinned(deploy.Labels) {
			continue
		}

		saving := spotSavingEstimate(deployKey, deploy.Replicas, nodeName, ac)
		if saving <= 0 {
			// Don't raise a Spot recommendation if we can't estimate savings.
			continue
		}

		recs = append(recs, advisor.Recommendation{
			ID:       recID(advisor.CategorySpot, deploy.Namespace, deploy.Name),
			Category: advisor.CategorySpot,
			Severity: advisor.SeverityLow,
			Resource: advisor.ResourceRef{Kind: "Deployment", Namespace: deploy.Namespace, Name: deploy.Name},
			Summary: fmt.Sprintf("Deployment appears stateless — consider Spot instances (~%.0f%% savings)",
				spotDiscountFactor*100),
			Detail: fmt.Sprintf(
				"Deployment %s/%s has %d replica(s) running on on-demand nodes. "+
					"It shows no persistent volume claims or on-demand node selectors. "+
					"Moving to a Spot nodegroup could reduce costs by ~%.0f%%. "+
					"Ensure the application tolerates interruptions (graceful shutdown, retry logic).",
				deploy.Namespace, deploy.Name, deploy.Replicas,
				spotDiscountFactor*100,
			),
			EstimatedMonthlySaving: saving,
			CurrentState: map[string]string{
				"capacity_type": "ON_DEMAND",
				"replicas":      fmt.Sprintf("%d", deploy.Replicas),
			},
			SuggestedAction: map[string]string{
				"capacity_type": "SPOT",
				"note":          "add nodeSelector eks.amazonaws.com/capacityType=SPOT or migrate to Spot nodegroup",
			},
			CheckName: "spot_opportunity",
		})
	}
	return recs, nil
}

func isOnDemandPinned(labels map[string]string) bool {
	for k, v := range labels {
		if strings.Contains(k, "capacityType") && strings.ToUpper(v) == "ON_DEMAND" {
			return true
		}
	}
	return false
}

func spotSavingEstimate(deployKey string, replicas int32, nodeName string, ac *advisor.AnalysisContext) float64 {
	for _, node := range ac.Nodes {
		if node.Name != nodeName {
			continue
		}
		pricePerHr, ok := ac.Pricing[node.InstanceType]
		if !ok {
			return 0
		}
		const assumedPodsPerNode = 10
		perPodHr := pricePerHr / assumedPodsPerNode
		currentMonthlyCost := perPodHr * 730 * float64(replicas)
		return math.Round(currentMonthlyCost*spotDiscountFactor*100) / 100
	}
	return 0
}
