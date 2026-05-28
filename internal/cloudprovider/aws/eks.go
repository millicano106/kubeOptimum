package aws

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/millicano106/kubeOptimum/internal/cloudprovider"
)

// EKSClient wraps the AWS EKS SDK client.
type EKSClient struct {
	svc         *eks.Client
	clusterName string
}

// NewEKSClient creates a new EKS client.
func NewEKSClient(cfg awssdk.Config, clusterName string) *EKSClient {
	return &EKSClient{
		svc:         eks.NewFromConfig(cfg),
		clusterName: clusterName,
	}
}

// FetchNodegroups lists all nodegroups in the cluster and returns summaries.
func (c *EKSClient) FetchNodegroups(ctx context.Context) ([]cloudprovider.NodegroupSummary, error) {
	names, err := c.listNodegroupNames(ctx)
	if err != nil {
		return nil, err
	}

	summaries := make([]cloudprovider.NodegroupSummary, 0, len(names))
	for _, name := range names {
		ng, err := c.describeNodegroup(ctx, name)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, ng)
	}
	return summaries, nil
}

func (c *EKSClient) listNodegroupNames(ctx context.Context) ([]string, error) {
	var names []string
	paginator := eks.NewListNodegroupsPaginator(c.svc, &eks.ListNodegroupsInput{
		ClusterName: awssdk.String(c.clusterName),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list nodegroups: %w", err)
		}
		names = append(names, page.Nodegroups...)
	}
	return names, nil
}

func (c *EKSClient) describeNodegroup(ctx context.Context, name string) (cloudprovider.NodegroupSummary, error) {
	out, err := c.svc.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
		ClusterName:   awssdk.String(c.clusterName),
		NodegroupName: awssdk.String(name),
	})
	if err != nil {
		return cloudprovider.NodegroupSummary{}, fmt.Errorf("describe nodegroup %s: %w", name, err)
	}
	ng := out.Nodegroup
	return nodegroupToSummary(c.clusterName, ng), nil
}

func nodegroupToSummary(clusterName string, ng *types.Nodegroup) cloudprovider.NodegroupSummary {
	var desired, min, max int32
	if ng.ScalingConfig != nil {
		if ng.ScalingConfig.DesiredSize != nil {
			desired = *ng.ScalingConfig.DesiredSize
		}
		if ng.ScalingConfig.MinSize != nil {
			min = *ng.ScalingConfig.MinSize
		}
		if ng.ScalingConfig.MaxSize != nil {
			max = *ng.ScalingConfig.MaxSize
		}
	}

	instanceType := ""
	if len(ng.InstanceTypes) > 0 {
		instanceType = ng.InstanceTypes[0]
	}

	labels := make(map[string]string)
	for k, v := range ng.Labels {
		labels[k] = v
	}

	return cloudprovider.NodegroupSummary{
		Name:         awssdk.ToString(ng.NodegroupName),
		ClusterName:  clusterName,
		InstanceType: instanceType,
		DesiredSize:  desired,
		MinSize:      min,
		MaxSize:      max,
		Labels:       labels,
	}
}
