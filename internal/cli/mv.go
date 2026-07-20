package cli

import (
	"github.com/spf13/cobra"
)

// newMvCmd builds the top-level `mv`, which moves or renames a node the way the
// shell's mv does — see store.Mv. It is silent on success.
func newMvCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mv <src> <dst>",
		Short: "Move or rename a node; a rename rewrites [[slug]] backlinks to follow",
		Long: "Move or rename a node, addressed as a slug path <parent>/<name> " +
			"(a bare <name> for an epic, _orphan/<name> for an orphan issue). " +
			"A changed parent moves the node; a changed name renames it, " +
			"rewriting [[slug]] backlinks; changing both does both.",
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
