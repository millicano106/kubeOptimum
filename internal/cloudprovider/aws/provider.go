package aws

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"

	"github.com/millicano106/kubeOptimum/internal/cloudprovider"
)

// Provider implements cloudprovider.CloudProvider for AWS EKS.
type Provider struct {
	eks     *EKSClient
	ec2     *EC2Client
	pricing *PricingClient
	dryRun  bool
}

// NewProvider builds an AWS Provider from an existing aws.Config.
func NewProvider(ctx context.Context, cfg awssdk.Config, clusterName string, dryRun bool) (*Provider, error) {
	var pc *PricingClient
	if !dryRun {
		var err error
		pc, err = NewPricingClientFromConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("pricing client: %w", err)
		}
	}
	return &Provider{
		eks:     NewEKSClient(cfg, clusterName),
		ec2:     NewEC2Client(cfg),
		pricing: pc,
		dryRun:  dryRun,
	}, nil
}

func (p *Provider) FetchNodegroups(ctx context.Context) ([]cloudprovider.NodegroupSummary, error) {
	return p.eks.FetchNodegroups(ctx)
}

func (p *Provider) FetchInstanceTypes(ctx context.Context, types []string) (map[string]cloudprovider.InstanceTypeDetail, error) {
	return p.ec2.FetchInstanceTypes(ctx, types)
}

func (p *Provider) FetchPricing(ctx context.Context, types []string) (map[string]float64, error) {
	if p.dryRun || p.pricing == nil {
		return nil, nil
	}
	return p.pricing.FetchOnDemandPrices(ctx, types)
}
