package prometheus

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// Fetcher collects all Prometheus metrics needed by the advisor in one call.
type Fetcher struct {
	API prometheusv1.API
}

// ContainerMetrics holds the metrics for a single container.
type ContainerMetrics struct {
	CPUP95Cores  float64 // p95 CPU cores
	MemP95Bytes  float64 // p95 memory working set in bytes
	CPUThrottled float64 // throttle ratio 0–1
}

// NodeMetrics holds the metrics for a single node.
type NodeMetrics struct {
	CPUUtil float64 // 0–1
	MemUtil float64 // 0–1
}

// FetchAll runs all needed Prometheus queries in sequence and returns the results.
// On partial failure the error is returned but populated maps are still usable.
func (f *Fetcher) FetchAll(ctx context.Context, namespace string, window time.Duration) (
	containers map[ContainerKey]ContainerMetrics,
	nodes map[NodeKey]NodeMetrics,
	err error,
) {
	slog.Debug("fetching prometheus metrics", "namespace", namespace, "window", window)
	win := WindowStr(window)
	ns := NSFilter(namespace)

	nsSelector := ns
	if nsSelector != "" {
		nsSelector = nsSelector + ","
	}

	containers = make(map[ContainerKey]ContainerMetrics)
	nodes = make(map[NodeKey]NodeMetrics)

	cpuQ := fmt.Sprintf(QueryCPUUsageP95, nsSelector, win)
	cpuData, cpuErr := QueryContainerFloat(ctx, f.API, cpuQ)
	if cpuErr != nil {
		return containers, nodes, fmt.Errorf("fetch CPU usage: %w", cpuErr)
	}

	memQ := fmt.Sprintf(QueryMemUsageP95, nsSelector, win)
	memData, memErr := QueryContainerFloat(ctx, f.API, memQ)
	if memErr != nil {
		return containers, nodes, fmt.Errorf("fetch memory usage: %w", memErr)
	}

	throttleQ := fmt.Sprintf(QueryCPUThrottleRatio, ns, win, ns, win)
	throttleData, _ := QueryContainerFloat(ctx, f.API, throttleQ)
	// Throttle is best-effort — don't fail the whole fetch if it errors.

	// Merge into ContainerMetrics map.
	keys := make(map[ContainerKey]struct{})
	for k := range cpuData {
		keys[k] = struct{}{}
	}
	for k := range memData {
		keys[k] = struct{}{}
	}
	for k := range keys {
		containers[k] = ContainerMetrics{
			CPUP95Cores:  cpuData[k],
			MemP95Bytes:  memData[k],
			CPUThrottled: throttleData[k],
		}
	}
	slog.Debug("container metrics fetched", "containers", len(containers))

	nodeCPU, nodeCPUErr := QueryNodeFloat(ctx, f.API, fmt.Sprintf(QueryNodeCPUUtil, win))
	if nodeCPUErr != nil {
		return containers, nodes, fmt.Errorf("fetch node CPU: %w", nodeCPUErr)
	}
	nodeMem, nodeMemErr := QueryNodeFloat(ctx, f.API, QueryNodeMemUtil)
	if nodeMemErr != nil {
		return containers, nodes, fmt.Errorf("fetch node memory: %w", nodeMemErr)
	}

	nodeKeys := make(map[NodeKey]struct{})
	for k := range nodeCPU {
		nodeKeys[k] = struct{}{}
	}
	for k := range nodeMem {
		nodeKeys[k] = struct{}{}
	}
	for k := range nodeKeys {
		nodes[k] = NodeMetrics{
			CPUUtil: nodeCPU[k],
			MemUtil: nodeMem[k],
		}
	}
	slog.Debug("node metrics fetched", "nodes", len(nodes))

	return containers, nodes, nil
}
