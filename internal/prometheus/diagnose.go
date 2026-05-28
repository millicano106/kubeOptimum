package prometheus

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// DiagnoseContainerMetrics runs a fast instant-query diagnostic when the main
// container metric queries returned nothing. It distinguishes between "the
// metric doesn't exist in Prometheus at all" and "the metric exists but the
// subquery returned empty" so the caller can give targeted advice.
func DiagnoseContainerMetrics(ctx context.Context, api prometheusv1.API, namespace string) string {
	ns := NSFilter(namespace)
	nsSelector := ns
	if nsSelector != "" {
		nsSelector = nsSelector + ","
	}

	// Simple instant count — much cheaper than the subquery, just checks existence.
	q := fmt.Sprintf(`count(container_cpu_usage_seconds_total{container!="",container!="POD",%s})`, nsSelector)
	slog.Debug("running container metric existence check", "query", q)

	result, _, err := api.Query(ctx, q, time.Now())
	if err != nil {
		return fmt.Sprintf("diagnostic query failed: %v — check Prometheus URL and connectivity", err)
	}

	vector, ok := result.(model.Vector)
	if !ok || len(vector) == 0 {
		if namespace != "" {
			return fmt.Sprintf(
				"container_cpu_usage_seconds_total has no data for namespace=%q.\n"+
					"      Either the namespace has no pods, the namespace label doesn't match,\n"+
					"      or kubelet/cAdvisor metrics are not being scraped by Prometheus.\n"+
					"      Try again without a namespace filter to see all namespaces.",
				namespace,
			)
		}
		return "container_cpu_usage_seconds_total has no data in Prometheus.\n" +
			"      Ensure kubelet metrics scraping is enabled (e.g. kube-prometheus-stack\n" +
			"      serviceMonitor for kubelet) and that cAdvisor is being scraped."
	}

	// Metric exists — the problem is the quantile_over_time subquery.
	count := int(vector[0].Value)
	return fmt.Sprintf(
		"container_cpu_usage_seconds_total EXISTS (%d active series) but the lookback query returned 0.\n"+
			"      Likely causes:\n"+
			"        • Lookback window is longer than available Prometheus retention / history.\n"+
			"          Try a shorter window: --window 1h or --window 24h\n"+
			"        • Prometheus scrape interval is too coarse for the 5m sub-range.\n"+
			"      Test query in Prometheus UI:\n"+
			"        rate(container_cpu_usage_seconds_total{container!=\"\",container!=\"POD\"}[5m])",
		count,
	)
}
