package advisor

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/kubernetes"

	"github.com/millicano106/kubeOptimum/internal/cloudprovider"
	"github.com/millicano106/kubeOptimum/internal/config"
	k8sclient "github.com/millicano106/kubeOptimum/internal/k8s"
	promclient "github.com/millicano106/kubeOptimum/internal/prometheus"
	"github.com/millicano106/kubeOptimum/pkg/timeutil"

	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// BuildContext fetches all three data sources in parallel and returns a ready
// AnalysisContext. Any single source failing aborts the entire fetch.
func BuildContext(ctx context.Context, cfg *config.Config, k8s kubernetes.Interface, promAPI prometheusv1.API, cloud cloudprovider.CloudProvider) (*AnalysisContext, error) {
	window, err := timeutil.ParseWindow(cfg.Analysis.LookbackWindow)
	if err != nil {
		return nil, fmt.Errorf("parse lookback window: %w", err)
	}

	ac := &AnalysisContext{
		LookbackWindow: window,
		Thresholds: Thresholds{
			CPURequestBufferPct:   cfg.Analysis.Thresholds.CPURequestBufferPct,
			MemRequestBufferPct:   cfg.Analysis.Thresholds.MemRequestBufferPct,
			IdleCPUThresholdMilli: cfg.Analysis.Thresholds.IdleCPUThresholdMilli,
		},
	}

	g, gctx := errgroup.WithContext(ctx)

	// K8s fetch.
	g.Go(func() error {
		var err error
		ac.Nodes, err = k8sclient.FetchNodes(gctx, k8s)
		if err != nil {
			return fmt.Errorf("fetch nodes: %w", err)
		}
		ac.Deployments, err = k8sclient.FetchDeployments(gctx, k8s, cfg.Analysis.Namespace)
		if err != nil {
			return fmt.Errorf("fetch deployments: %w", err)
		}
		ac.Pods, err = k8sclient.FetchPods(gctx, k8s, cfg.Analysis.Namespace)
		if err != nil {
			return fmt.Errorf("fetch pods: %w", err)
		}
		return nil
	})

	// Prometheus fetch.
	g.Go(func() error {
		fetcher := &promclient.Fetcher{API: promAPI}
		containerMetrics, nodeMetrics, err := fetcher.FetchAll(gctx, cfg.Analysis.Namespace, window)
		if err != nil {
			return fmt.Errorf("fetch prometheus metrics: %w", err)
		}
		ac.ContainerMetrics = containerMetrics
		ac.NodeMetrics = nodeMetrics
		return nil
	})

	// Cloud provider fetch.
	g.Go(func() error {
		nodegroups, err := cloud.FetchNodegroups(gctx)
		if err != nil {
			return fmt.Errorf("fetch nodegroups: %w", err)
		}
		ac.Nodegroups = nodegroups

		// Collect unique instance types from nodegroups.
		typeSet := make(map[string]struct{})
		for _, ng := range nodegroups {
			if ng.InstanceType != "" {
				typeSet[ng.InstanceType] = struct{}{}
			}
		}
		types := make([]string, 0, len(typeSet))
		for t := range typeSet {
			types = append(types, t)
		}

		instanceTypes, err := cloud.FetchInstanceTypes(gctx, types)
		if err != nil {
			return fmt.Errorf("fetch instance types: %w", err)
		}
		ac.InstanceTypes = instanceTypes

		pricing, err := cloud.FetchPricing(gctx, types)
		if err != nil {
			return fmt.Errorf("fetch pricing: %w", err)
		}
		ac.Pricing = pricing
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Backfill instance types found on nodes (may differ from nodegroup listing).
	if ac.InstanceTypes == nil {
		ac.InstanceTypes = make(map[string]cloudprovider.InstanceTypeDetail)
	}

	return ac, nil
}
