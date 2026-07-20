package cli

import (
	"github.com/spf13/cobra"
)

func newRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <ref>",
		Short: "Remove a node; an epic or issue takes its whole subtree",
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
			return st.Remove(loc)
		},
	}
}
