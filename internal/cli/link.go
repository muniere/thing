package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/render"
)

func newLinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Manage a node's related links",
	}
	cmd.AddCommand(newLinkAddCmd(), newLinkRmCmd(), newLinkListCmd())
	return cmd
}

func newLinkAddCmd() *cobra.Command {
	var label string
	cmd := &cobra.Command{
		Use:   "add <ref> <url>",
		Short: "Add a related link, or update its label when the URL exists",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(cmd)
			if err != nil {
				return err
			}
			loc, err := st.Get(args[0])
			if err != nil {
				return err
			}
			return st.AddLink(loc, args[1], label, today())
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "link label")
	return cmd
}

func newLinkRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <ref> <url|index>",
		Short: "Remove a related link by URL, or by its 1-based index",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(cmd)
			if err != nil {
				return err
			}
			loc, err := st.Get(args[0])
			if err != nil {
				return err
			}
			return st.RemoveLink(loc, args[1], today())
		},
	}
}

func newLinkListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <ref>",
		Short: "List a node's related links",
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
			fmt.Fprint(cmd.OutOrStdout(), render.Links(loc.Node.Links))
			return nil
		},
	}
}
