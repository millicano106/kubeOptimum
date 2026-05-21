package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/millicano106/kubeOptimum/internal/config"
)

var (
	cfgFile string
	v       = viper.New()
)

var rootCmd = &cobra.Command{
	Use:   "kubeOptimum",
	Short: "Kubernetes cost optimisation advisor",
	Long: `kubeOptimum connects to your Kubernetes cluster, Prometheus stack, and AWS EKS
to analyse resource usage and suggest concrete cost optimisations.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.kubeOptimum/config.yaml)")
	rootCmd.PersistentFlags().String("kubeconfig", "", "path to kubeconfig file")
	rootCmd.PersistentFlags().String("context", "", "kubeconfig context to use")
	rootCmd.PersistentFlags().String("namespace", "", "Kubernetes namespace (empty = all)")
	rootCmd.PersistentFlags().String("output", "table", "output format: table, json, yaml")
	rootCmd.PersistentFlags().String("window", "7d", "Prometheus lookback window (e.g. 24h, 7d)")
	rootCmd.PersistentFlags().Bool("dry-run", false, "skip AWS Pricing API calls")

	_ = v.BindPFlag("kubernetes.kubeconfig", rootCmd.PersistentFlags().Lookup("kubeconfig"))
	_ = v.BindPFlag("kubernetes.context", rootCmd.PersistentFlags().Lookup("context"))
	_ = v.BindPFlag("analysis.namespace", rootCmd.PersistentFlags().Lookup("namespace"))
	_ = v.BindPFlag("analysis.outputFormat", rootCmd.PersistentFlags().Lookup("output"))
	_ = v.BindPFlag("analysis.lookbackWindow", rootCmd.PersistentFlags().Lookup("window"))

	v.SetEnvPrefix("KUBEOPTIMUM")
	v.AutomaticEnv()
}

func initConfig() {
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigFile(config.DefaultConfigPath())
	}
	// Ignore missing config file — defaults are fine.
	_ = v.ReadInConfig()
}

// loadConfig returns the resolved Config for use by subcommands.
func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(v)
	if err != nil {
		return nil, err
	}
	// Ensure config dir exists for first-run scenarios.
	dir := filepath.Dir(config.DefaultConfigPath())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	return cfg, nil
}
