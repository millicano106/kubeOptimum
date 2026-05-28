package advisor

import (
	"time"

	"github.com/millicano106/kubeOptimum/internal/cloudprovider"
	"github.com/millicano106/kubeOptimum/internal/k8s"
	prommetrics "github.com/millicano106/kubeOptimum/internal/prometheus"
)

// AnalysisContext is a read-only snapshot of all data collected from the three
// data sources. It is built once per analysis run and passed to every Check.
type AnalysisContext struct {
	// Kubernetes
	Nodes       []k8s.NodeSummary
	Deployments []k8s.DeploymentSummary
	Pods        []k8s.PodSummary

	// Prometheus — keyed by ContainerKey or NodeKey
	ContainerMetrics map[prommetrics.ContainerKey]prommetrics.ContainerMetrics
	NodeMetrics      map[prommetrics.NodeKey]prommetrics.NodeMetrics

	// Cloud provider
	Nodegroups    []cloudprovider.NodegroupSummary
	InstanceTypes map[string]cloudprovider.InstanceTypeDetail // instanceType → detail
	Pricing       map[string]float64                          // instanceType → $/hr

	// Configuration passed through from the CLI
	LookbackWindow time.Duration
	Thresholds     Thresholds

	// Diagnostics holds human-readable messages produced during data collection
	// (e.g. "metric exists but query returned 0 — try a shorter --window").
	Diagnostics []string
}

// Thresholds mirrors config.Thresholds but lives in the advisor package so
// checks don't import the config package.
type Thresholds struct {
	CPURequestBufferPct   float64
	MemRequestBufferPct   float64
	IdleCPUThresholdMilli float64
}
