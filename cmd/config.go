package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/millicano106/kubeOptimum/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage kubeOptimum configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a default config file to ~/.kubeOptimum/config.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := config.DefaultConfigPath()
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("config already exists at %s\n", path)
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}
		if err := os.WriteFile(path, []byte(config.DefaultYAML), 0o600); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
		fmt.Printf("config written to %s\n", path)
		return nil
	},
}

var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Print the resolved configuration as YAML",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		out, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("marshal config: %w", err)
		}
		fmt.Print(string(out))
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configViewCmd)
	rootCmd.AddCommand(configCmd)
}
