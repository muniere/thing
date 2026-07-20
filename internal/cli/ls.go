package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/config"
	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/render"
	"github.com/muniere/thing/internal/store"
)

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls [<ref>]",
		Short: "List a node's children, or the top level when omitted",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(cmd)
			if err != nil {
				return err
			}

			// The top level (epics and orphan issues) is grouped under the
			// configured category headings.
			if len(args) == 0 {
				top, err := st.Load()
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
				fmt.Fprint(cmd.OutOrStdout(), render.TopList(top, cfg.Categories))
				return nil
			}

			var nodes []*model.Node
			if args[0] == store.OrphanDir {
				top, err := st.Load()
				if err != nil {
					return err
				}
				for _, n := range top {
					if n.Type == model.Issue {
						nodes = append(nodes, n)
					}
				}
			} else {
				loc, err := st.Get(args[0])
				if err != nil {
					return err
				}
				nodes = loc.Node.Children
			}
			fmt.Fprint(cmd.OutOrStdout(), render.List(nodes))
			return nil
		},
	}
}
