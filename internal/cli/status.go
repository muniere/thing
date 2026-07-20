package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/model"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <ref> <status>",
		Short: "Set a node's status",
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
			s := model.Status(args[1])
			if !s.Valid() {
				return fmt.Errorf("invalid status %q (want %s)", args[1], model.StatusValues())
			}
			loc.Node.Status = s
			loc.Node.Updated = today()
			return st.Save(loc)
		},
	}
}

func newPriorityCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "priority <ref> <priority>",
		Short: "Set a node's priority",
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
			p := model.Priority(args[1])
			if !p.Valid() {
				return fmt.Errorf("invalid priority %q (want %s)", args[1], model.PriorityValues())
			}
			loc.Node.Priority = p
			loc.Node.Updated = today()
			return st.Save(loc)
		},
	}
}
