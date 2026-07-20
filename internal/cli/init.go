package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/config"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create the data and config directories and a starter config.yaml",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := initDataDir(cmd)
			if err != nil {
				return err
			}
			cfg, err := initConfigDir(cmd)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(data, 0o755); err != nil {
				return err
			}
			if err := os.MkdirAll(cfg, 0o755); err != nil {
				return err
			}
			cfgPath := filepath.Join(cfg, config.FileName)
			if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
				if err := os.WriteFile(cfgPath, config.Starter(), 0o644); err != nil {
					return err
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "initialized data %s\n", data)
			fmt.Fprintf(cmd.OutOrStdout(), "initialized config %s\n", cfg)
			return nil
		},
	}
}
