package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/render"
	"github.com/muniere/thing/internal/store"
)

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls [<parent>]",
		Short: "List a parent's children, or the top level when omitted",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(cmd)
			if err != nil {
				return err
			}
			var nodes []*model.Node
			switch {
			case len(args) == 0:
				// The top level: epics and orphan issues.
				nodes, err = st.Load()
			case args[0] == store.OrphanDir:
				var top []*model.Node
				top, err = st.Load()
				for _, n := range top {
					if err == nil && n.Type == model.Issue {
						nodes = append(nodes, n)
					}
				}
			default:
				var loc *store.Entry
				loc, err = st.Get(args[0])
				if err == nil {
					nodes = loc.Node.Children
				}
			}
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), render.List(nodes))
			return nil
		},
	}
}
