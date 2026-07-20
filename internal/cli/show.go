package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/render"
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
			loc, err := st.Get(args[0])
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), render.Show(loc.Node))
			return nil
		},
	}
}
