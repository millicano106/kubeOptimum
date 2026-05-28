package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/millicano106/kubeOptimum/internal/advisor"
	awsprovider "github.com/millicano106/kubeOptimum/internal/cloudprovider/aws"
	k8sclient "github.com/millicano106/kubeOptimum/internal/k8s"
	"github.com/millicano106/kubeOptimum/internal/output"
	promclient "github.com/millicano106/kubeOptimum/internal/prometheus"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Run analysis and save a timestamped report file",
	RunE:  runReport,
}

func init() {
	reportCmd.Flags().String("output-file", "./", "directory or full path for the report file")
	reportCmd.Flags().StringSlice("filter-category", nil, "only show these categories")
	reportCmd.Flags().StringSlice("filter-severity", nil, "only show these severities (HIGH, MEDIUM, LOW)")
	reportCmd.Flags().StringSlice("disable-check", nil, "skip these checks by name (rightsizing, idle_workloads, nodegroup_sizing, instance_type, spot_opportunity)")
	rootCmd.AddCommand(reportCmd)
}

func runReport(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	ctx := cmd.Context()

	k8s, err := k8sclient.NewClient(cfg.Kubernetes.Kubeconfig, cfg.Kubernetes.Context, cfg.Kubernetes.InCluster)
	if err != nil {
		return fmt.Errorf("kubernetes client: %w", err)
	}

	promAPI, err := promclient.NewAPI(cfg.Prometheus.URL, cfg.Prometheus.Token, cfg.Prometheus.Username, cfg.Prometheus.Password)
	if err != nil {
		return fmt.Errorf("prometheus client: %w", err)
	}

	dryRun, _ := rootCmd.PersistentFlags().GetBool("dry-run")
	awsCfg, err := awsprovider.NewConfig(ctx, cfg.AWS.Region, cfg.AWS.Profile, cfg.AWS.RoleARN)
	if err != nil {
		return fmt.Errorf("aws config: %w", err)
	}
	cloud, err := awsprovider.NewProvider(ctx, awsCfg, cfg.AWS.ClusterName, dryRun)
	if err != nil {
		return fmt.Errorf("aws provider: %w", err)
	}

	ac, err := advisor.BuildContext(ctx, cfg, k8s, promAPI, cloud)
	if err != nil {
		return fmt.Errorf("build analysis context: %w", err)
	}
	printDataSummary(os.Stderr, ac)

	disabled, _ := cmd.Flags().GetStringSlice("disable-check")
	adv := advisor.NewAdvisor(enabledChecks(disabled))
	result := adv.Run(ctx, ac)

	for _, e := range result.Errors {
		fmt.Fprintf(os.Stderr, "warning: %v\n", e)
	}

	recs := applyFilters(result.Recommendations, cmd)

	// Resolve output path.
	outPath, _ := cmd.Flags().GetString("output-file")
	format, _ := rootCmd.PersistentFlags().GetString("output")
	if format == "" {
		format = "json"
	}
	ext := format
	if ext == "table" {
		ext = "txt"
	}

	info, err := os.Stat(outPath)
	if err == nil && info.IsDir() {
		date := time.Now().Format("2006-01-02")
		outPath = filepath.Join(outPath, fmt.Sprintf("kubeOptimum-report-%s.%s", date, ext))
	}

	f, err := os.Create(filepath.Clean(outPath)) // #nosec G304 -- output path is intentionally user-supplied
	if err != nil {
		return fmt.Errorf("create report file: %w", err)
	}
	defer f.Close()

	renderer, err := output.NewRenderer(format)
	if err != nil {
		return err
	}
	if err := renderer.Render(f, recs); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Report written to %s (%d recommendations)\n", outPath, len(recs))
	return nil
}
