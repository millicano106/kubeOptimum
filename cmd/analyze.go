package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/millicano106/kubeOptimum/internal/advisor"
	"github.com/millicano106/kubeOptimum/internal/advisor/checks"
	awsprovider "github.com/millicano106/kubeOptimum/internal/cloudprovider/aws"
	k8sclient "github.com/millicano106/kubeOptimum/internal/k8s"
	"github.com/millicano106/kubeOptimum/internal/output"
	promclient "github.com/millicano106/kubeOptimum/internal/prometheus"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyse cluster resources and print cost optimisation recommendations",
	RunE:  runAnalyze,
}

func init() {
	analyzeCmd.Flags().StringSlice("filter-category", nil, "only show these categories (RIGHT_SIZING, IDLE_WORKLOAD, NODEGROUP, INSTANCE_TYPE, SPOT_OPPORTUNITY)")
	analyzeCmd.Flags().StringSlice("filter-severity", nil, "only show these severities (HIGH, MEDIUM, LOW)")
	analyzeCmd.Flags().StringSlice("disable-check", nil, "skip these checks by name (rightsizing, idle_workloads, nodegroup_sizing, instance_type, spot_opportunity)")
	rootCmd.AddCommand(analyzeCmd)
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	ctx := cmd.Context()

	// Build K8s client.
	k8s, err := k8sclient.NewClient(cfg.Kubernetes.Kubeconfig, cfg.Kubernetes.Context, cfg.Kubernetes.InCluster)
	if err != nil {
		return fmt.Errorf("kubernetes client: %w", err)
	}

	// Build Prometheus API client.
	promAPI, err := promclient.NewAPI(cfg.Prometheus.URL, cfg.Prometheus.Token, cfg.Prometheus.Username, cfg.Prometheus.Password)
	if err != nil {
		return fmt.Errorf("prometheus client: %w", err)
	}

	// Build AWS cloud provider.
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if !dryRun {
		dryRun, _ = rootCmd.PersistentFlags().GetBool("dry-run")
	}
	awsCfg, err := awsprovider.NewConfig(ctx, cfg.AWS.Region, cfg.AWS.Profile, cfg.AWS.RoleARN)
	if err != nil {
		return fmt.Errorf("aws config: %w", err)
	}
	cloud, err := awsprovider.NewProvider(ctx, awsCfg, cfg.AWS.ClusterName, dryRun)
	if err != nil {
		return fmt.Errorf("aws provider: %w", err)
	}

	// Collect all data in parallel.
	ac, err := advisor.BuildContext(ctx, cfg, k8s, promAPI, cloud)
	if err != nil {
		return fmt.Errorf("build analysis context: %w", err)
	}
	printDataSummary(os.Stderr, ac)

	// Run all advisor checks (minus any explicitly disabled ones).
	disabled, _ := cmd.Flags().GetStringSlice("disable-check")
	adv := advisor.NewAdvisor(enabledChecks(disabled))
	result := adv.Run(ctx, ac)

	for _, e := range result.Errors {
		fmt.Fprintf(os.Stderr, "warning: %v\n", e)
	}

	recs := applyFilters(result.Recommendations, cmd)

	// Render output.
	format, _ := rootCmd.PersistentFlags().GetString("output")
	renderer, err := output.NewRenderer(format)
	if err != nil {
		return err
	}
	return renderer.Render(os.Stdout, recs)
}

func applyFilters(recs []advisor.Recommendation, cmd *cobra.Command) []advisor.Recommendation {
	cats, _ := cmd.Flags().GetStringSlice("filter-category")
	sevs, _ := cmd.Flags().GetStringSlice("filter-severity")

	if len(cats) == 0 && len(sevs) == 0 {
		return recs
	}

	catSet := toSet(cats)
	sevSet := toSet(sevs)

	filtered := recs[:0]
	for _, r := range recs {
		if len(catSet) > 0 && !catSet[string(r.Category)] {
			continue
		}
		if len(sevSet) > 0 && !sevSet[string(r.Severity)] {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func enabledChecks(disabled []string) []advisor.Check {
	disabledSet := toSet(disabled)
	all := []advisor.Check{
		checks.RightSizing{},
		checks.IdleWorkloads{},
		checks.NodegroupSizing{},
		checks.InstanceType{},
		checks.SpotOpportunity{},
	}
	enabled := all[:0]
	for _, c := range all {
		if !disabledSet[c.Name()] {
			enabled = append(enabled, c)
		}
	}
	return enabled
}
