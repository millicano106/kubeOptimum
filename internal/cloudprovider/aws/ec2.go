package aws

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/millicano106/kubeOptimum/internal/cloudprovider"
)

// EC2Client wraps the AWS EC2 SDK client for instance type queries.
type EC2Client struct {
	svc *ec2.Client
}

// NewEC2Client creates a new EC2 client.
func NewEC2Client(cfg awssdk.Config) *EC2Client {
	return &EC2Client{svc: ec2.NewFromConfig(cfg)}
}

// FetchInstanceTypes retrieves vCPU and memory details for the requested types.
func (c *EC2Client) FetchInstanceTypes(ctx context.Context, types []string) (map[string]cloudprovider.InstanceTypeDetail, error) {
	if len(types) == 0 {
		return nil, nil
	}

	itypes := make([]ec2types.InstanceType, len(types))
	for i, t := range types {
		itypes[i] = ec2types.InstanceType(t)
	}

	out, err := c.svc.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: itypes,
	})
	if err != nil {
		return nil, fmt.Errorf("describe instance types: %w", err)
	}

	result := make(map[string]cloudprovider.InstanceTypeDetail, len(out.InstanceTypes))
	for _, it := range out.InstanceTypes {
		name := string(it.InstanceType)
		detail := cloudprovider.InstanceTypeDetail{Name: name}
		if it.VCpuInfo != nil && it.VCpuInfo.DefaultVCpus != nil {
			detail.VCPUs = *it.VCpuInfo.DefaultVCpus
		}
		if it.MemoryInfo != nil && it.MemoryInfo.SizeInMiB != nil {
			detail.MemoryMiB = *it.MemoryInfo.SizeInMiB
		}
		result[name] = detail
	}
	return result, nil
}

// SameFamily returns instance types in the same family as the given type with
// fewer vCPUs (cheaper alternatives within the same generation).
// e.g. "m5.2xlarge" → ["m5.xlarge", "m5.large"]
func SameFamily(instanceType string, all map[string]cloudprovider.InstanceTypeDetail, current cloudprovider.InstanceTypeDetail) []cloudprovider.InstanceTypeDetail {
	family := instanceFamily(instanceType)
	var candidates []cloudprovider.InstanceTypeDetail
	for name, detail := range all {
		if instanceFamily(name) == family && detail.VCPUs < current.VCPUs {
			candidates = append(candidates, detail)
		}
	}
	return candidates
}

// instanceFamily extracts the family prefix from an instance type string.
// "m5.2xlarge" → "m5", "c6g.large" → "c6g"
func instanceFamily(instanceType string) string {
	for i, ch := range instanceType {
		if ch == '.' {
			return instanceType[:i]
		}
	}
	return instanceType
}
