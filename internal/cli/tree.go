package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/config"
	"github.com/muniere/thing/internal/render"
)

func newTreeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tree",
		Short: "Print the whole tree",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			st, err := openStore(cmd)
			if err != nil {
				return err
			}
			nodes, err := st.Load()
			if err != nil {
				return err
			}
			cfgDir, err := configDir(cmd)
			if err != nil {
				return err
			}
			cfg, err := config.Load(cfgDir)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), render.Tree(nodes, cfg.Title))
			return nil
		},
	}
}
