package prometheus

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// PromQL query templates. LOOKBACK and NS_FILTER are replaced at call time.
const (
	// QueryCPUUsageP95 returns p95 CPU cores per {namespace, pod, container}.
	QueryCPUUsageP95 = `quantile_over_time(0.95,
  rate(container_cpu_usage_seconds_total{container!="",container!="POD",%s}[5m])[%s:5m])`

	// QueryMemUsageP95 returns p95 memory bytes per {namespace, pod, container}.
	QueryMemUsageP95 = `quantile_over_time(0.95,
  container_memory_working_set_bytes{container!="",container!="POD",%s}[%s:5m])`

	// QueryCPUThrottleRatio returns throttling ratio per {namespace, pod, container}.
	QueryCPUThrottleRatio = `sum by (namespace, pod, container) (
  rate(container_cpu_cfs_throttled_seconds_total{%s}[%s])
) / sum by (namespace, pod, container) (
  rate(container_cpu_cfs_periods_total{%s}[%s])
)`

	// QueryNodeCPUUtil returns CPU utilisation ratio per node.
	QueryNodeCPUUtil = `1 - avg by (node) (rate(node_cpu_seconds_total{mode="idle"}[%s]))`

	// QueryNodeMemUtil returns memory utilisation ratio per node.
	QueryNodeMemUtil = `1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)`
)

// ContainerKey is the label set used to key per-container metric results.
type ContainerKey struct {
	Namespace string
	Pod       string
	Container string
}

// NodeKey is the label set used to key per-node metric results.
type NodeKey struct {
	Node string
}

// QueryContainerFloat runs an instant query and returns a map of ContainerKey → float64.
func QueryContainerFloat(ctx context.Context, api prometheusv1.API, query string) (map[ContainerKey]float64, error) {
	start := time.Now()
	slog.Debug("prometheus query", "query", query)

	result, _, err := api.Query(ctx, query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("prometheus query: %w", err)
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("unexpected result type %T", result)
	}

	out := make(map[ContainerKey]float64, len(vector))
	for _, sample := range vector {
		key := ContainerKey{
			Namespace: string(sample.Metric["namespace"]),
			Pod:       string(sample.Metric["pod"]),
			Container: string(sample.Metric["container"]),
		}
		out[key] = float64(sample.Value)
	}
	slog.Debug("prometheus query done", "samples", len(out), "elapsed", time.Since(start))
	return out, nil
}

// QueryNodeFloat runs an instant query and returns a map of node name → float64.
func QueryNodeFloat(ctx context.Context, api prometheusv1.API, query string) (map[NodeKey]float64, error) {
	start := time.Now()
	slog.Debug("prometheus node query", "query", query)

	result, _, err := api.Query(ctx, query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("prometheus query: %w", err)
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("unexpected result type %T", result)
	}

	out := make(map[NodeKey]float64, len(vector))
	for _, sample := range vector {
		key := NodeKey{Node: string(sample.Metric["node"])}
		out[key] = float64(sample.Value)
	}
	slog.Debug("prometheus node query done", "samples", len(out), "elapsed", time.Since(start))
	return out, nil
}

// NSFilter builds a Prometheus label selector for namespace.
// Returns an empty string when namespace is "" (all namespaces).
func NSFilter(namespace string) string {
	if namespace == "" {
		return ""
	}
	return fmt.Sprintf(`namespace="%s"`, namespace)
}

// WindowStr formats a duration as a Prometheus range string (e.g. "7d", "24h").
func WindowStr(d time.Duration) string {
	if d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", int(d.Hours())/24)
	}
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
