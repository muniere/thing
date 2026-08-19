package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/render"
)

func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check [<ref>]",
		Short: "Validate a node's body against the section convention",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(cmd)
			if err != nil {
				return err
			}

			var items []render.CheckItem
			if len(args) == 1 {
				loc, err := st.Get(args[0])
				if err != nil {
					return err
				}
				items = []render.CheckItem{{Ref: loc.Ref, Markers: loc.Node.Markers()}}
			} else {
				idx, err := st.Index()
				if err != nil {
					return err
				}
				refs := make([]string, 0, len(idx))
				for ref := range idx {
					refs = append(refs, ref)
				}
				sort.Strings(refs)
				items = make([]render.CheckItem, len(refs))
				for i, ref := range refs {
					items[i] = render.CheckItem{Ref: ref, Markers: idx[ref].Node.Markers()}
				}
			}

			// Warnings never fail the command — a non-zero exit would break
			// scripts that call `thing check` routinely.
			fmt.Fprint(cmd.OutOrStdout(), render.Check(items))
			return nil
		},
	}
}
