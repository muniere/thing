package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newArchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "archive <ref>",
		Short: "Archive a node; an epic or issue takes its whole subtree",
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
			ref, err := st.Archive(loc, now())
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), ref)
			return nil
		},
	}
}

func newUnarchiveCmd() *cobra.Command {
	var to string
	cmd := &cobra.Command{
		Use:   "unarchive <archive-ref>",
		Short: "Restore an archived node to the live tree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(cmd)
			if err != nil {
				return err
			}
			ae, err := st.ArchiveGet(args[0])
			if err != nil {
				return err
			}
			ref, err := st.Unarchive(ae, to, today())
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), ref)
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "restore to this ref instead of where it was archived from")
	return cmd
}
