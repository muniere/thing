package cli

import (
	"github.com/spf13/cobra"
)

// newMvCmd builds the top-level `mv`, which moves or renames a node the way the
// shell's mv does — see store.Mv. It is silent on success.
func newMvCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mv <src> <dst>",
		Short: "Move or rename a node; a rename rewrites [[ref]] backlinks to follow",
		Long: "Move or rename a node. src and dst are refs " +
			"(<epic>, <epic>/<issue>, <epic>/<issue>/<task>; _orphan/<issue> for " +
			"an orphan). A changed parent moves the node; a changed name renames " +
			"it, rewriting [[ref]] backlinks; changing both does both.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(cmd)
			if err != nil {
				return err
			}
			return st.Mv(args[0], args[1], today())
		},
	}
}
