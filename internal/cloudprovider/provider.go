package cloudprovider

import "context"

// NodegroupSummary describes a managed nodegroup (EKS, GKE node pool, etc.).
type NodegroupSummary struct {
	Name         string
	ClusterName  string
	InstanceType string
	DesiredSize  int32
	MinSize      int32
	MaxSize      int32
	Labels       map[string]string
	// TotalCPUMilli and TotalMemBytes are computed as desired * instance capacity.
	TotalCPUMilli int64
	TotalMemBytes int64
}

// InstanceTypeDetail holds vCPU and memory for one EC2/GCE/Azure instance type.
type InstanceTypeDetail struct {
	Name      string
	VCPUs     int32
	MemoryMiB int64
}

// CloudProvider abstracts the cloud-specific data fetching so the advisor
// works identically whether the cluster runs on AWS, GCP, or Azure.
type CloudProvider interface {
	FetchNodegroups(ctx context.Context) ([]NodegroupSummary, error)
	FetchInstanceTypes(ctx context.Context, types []string) (map[string]InstanceTypeDetail, error)
	FetchPricing(ctx context.Context, types []string) (map[string]float64, error)
}
