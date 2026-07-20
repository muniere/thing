// Package cli defines the `thing` command tree, built on Cobra. The cmd/thing
// binary is a thin entry point that calls NewRootCmd.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/store"
)

// NewRootCmd builds the top-level `thing` command and wires its subcommands.
// version is injected by the binary so it can be overridden via -ldflags.
func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "thing",
		Short:         "Manage a topic tree of Epic > Issue > Task nodes",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentFlags().String("data-dir", "", "data directory (default: nearest .thing/ searched upward)")
	cmd.PersistentFlags().String("config", "", "config directory (default: nearest .thing/ upward, else ~/.config/thing)")
	cmd.PersistentFlags().BoolP("global", "g", false, "use the global directories (~/.local/share/thing and ~/.config/thing)")
	cmd.AddCommand(
		newInitCmd(),
		newTreeCmd(),
		newEpicCmd(),
		newIssueCmd(),
		newTaskCmd(),
	)
	return cmd
}

// dataDir resolves the tree's data directory from the persistent flags.
func dataDir(cmd *cobra.Command) (string, error) {
	flag, _ := cmd.Flags().GetString("data-dir")
	global, _ := cmd.Flags().GetBool("global")
	return store.DataDir(flag, global)
}

// configDir resolves the config directory from the persistent flags.
func configDir(cmd *cobra.Command) (string, error) {
	flag, _ := cmd.Flags().GetString("config")
	global, _ := cmd.Flags().GetBool("global")
	return store.ConfigDir(flag, global)
}

// openStore opens the store rooted at the resolved data directory.
func openStore(cmd *cobra.Command) (*store.Store, error) {
	dir, err := dataDir(cmd)
	if err != nil {
		return nil, err
	}
	return store.Open(dir), nil
}
