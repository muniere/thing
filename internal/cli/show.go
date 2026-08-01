package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/render"
	"github.com/muniere/thing/internal/store"
)

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <ref>",
		Short: "Show a node and its body",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(cmd)
			if err != nil {
				return err
			}
			// An archived node is addressed by its "_archives/<name>" ref and lives
			// outside the live tree, so it is looked up through the archive API.
			if args[0] == store.ArchiveDir || strings.HasPrefix(args[0], store.ArchiveDir+"/") {
				ae, err := st.ArchiveGet(args[0])
				if err != nil {
					return err
				}
				fmt.Fprint(cmd.OutOrStdout(), render.Show(ae.Node))
				return nil
			}
			loc, err := st.Get(args[0])
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), render.Show(loc.Node))
			return nil
		},
	}
}
