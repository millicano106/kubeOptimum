package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	Kubernetes KubernetesConfig `mapstructure:"kubernetes"`
	Prometheus PrometheusConfig `mapstructure:"prometheus"`
	AWS        AWSConfig        `mapstructure:"aws"`
	Analysis   AnalysisConfig   `mapstructure:"analysis"`
}

type KubernetesConfig struct {
	Kubeconfig string `mapstructure:"kubeconfig"`
	Context    string `mapstructure:"context"`
	InCluster  bool   `mapstructure:"inCluster"`
}

type PrometheusConfig struct {
	URL      string `mapstructure:"url"`
	Token    string `mapstructure:"token"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type AWSConfig struct {
	Region      string `mapstructure:"region"`
	Profile     string `mapstructure:"profile"`
	RoleARN     string `mapstructure:"roleARN"`
	ClusterName string `mapstructure:"clusterName"`
}

type AnalysisConfig struct {
	LookbackWindow string     `mapstructure:"lookbackWindow"`
	Namespace      string     `mapstructure:"namespace"`
	OutputFormat   string     `mapstructure:"outputFormat"`
	Thresholds     Thresholds `mapstructure:"thresholds"`
}

type Thresholds struct {
	CPURequestBufferPct   float64 `mapstructure:"cpuRequestBufferPct"`
	MemRequestBufferPct   float64 `mapstructure:"memRequestBufferPct"`
	IdleCPUThresholdMilli float64 `mapstructure:"idleCPUThresholdMilli"`
}

func Defaults() *Config {
	return &Config{
		Prometheus: PrometheusConfig{
			URL: "http://localhost:9090",
		},
		AWS: AWSConfig{
			Region: "us-east-1",
		},
		Analysis: AnalysisConfig{
			LookbackWindow: "7d",
			OutputFormat:   "table",
			Thresholds: Thresholds{
				CPURequestBufferPct:   20,
				MemRequestBufferPct:   20,
				IdleCPUThresholdMilli: 5,
			},
		},
	}
}

func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kubeOptimum", "config.yaml")
}

func Load(v *viper.Viper) (*Config, error) {
	cfg := Defaults()
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return cfg, nil
}

func Validate(cfg *Config) error {
	if cfg.Prometheus.URL == "" {
		return fmt.Errorf("prometheus.url is required")
	}
	if cfg.Analysis.LookbackWindow == "" {
		return fmt.Errorf("analysis.lookbackWindow is required")
	}
	return nil
}

const DefaultYAML = `kubernetes:
  kubeconfig: ""
  context: ""
  inCluster: false

prometheus:
  url: "http://localhost:9090"
  token: ""
  username: ""
  password: ""

aws:
  region: "us-east-1"
  profile: ""
  roleARN: ""
  clusterName: ""

analysis:
  lookbackWindow: "7d"
  namespace: ""
  outputFormat: "table"
  thresholds:
    cpuRequestBufferPct: 20
    memRequestBufferPct: 20
    idleCPUThresholdMilli: 5
`
