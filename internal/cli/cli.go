// Package cli defines the `thing` command tree, built on Cobra. The cmd/thing
// binary is a thin entry point that calls NewRootCmd.
package cli

import (
	"errors"
	"os"
	"strings"
	"time"

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
		newAddCmd(),
		newLsCmd(),
		newShowCmd(),
		newStatusCmd(),
		newPriorityCmd(),
		newMvCmd(),
		newRmCmd(),
		newLinkCmd(),
		newTreeCmd(),
	)
	return cmd
}

// openStore opens the store rooted at the resolved data directory.
func openStore(cmd *cobra.Command) (*store.Store, error) {
	dir, err := dataDir(cmd)
	if err != nil {
		return nil, err
	}
	return store.Open(dir), nil
}

func today() string {
	return time.Now().Format("2006-01-02")
}

// Environment variables that override the data and config directories.
const (
	envDataDir   = "THING_DATA_DIR"
	envConfigDir = "THING_CONFIG_DIR"
)

// Directory resolution combines CLI inputs (the --data-dir / --config flags and
// -g) with the THING_*_DIR env vars and the store's directory primitives. This
// precedence lives in the CLI layer; the store only knows how to find a project
// .thing/ and where the global XDG directories are.

// dataDir resolves the tree's data directory:
//
//	--data-dir  ->  THING_DATA_DIR  ->  -g global  ->  nearest .thing/ upward
//
// There is no implicit global fallback: it errors rather than silently use a
// global tree.
func dataDir(cmd *cobra.Command) (string, error) {
	if flag, _ := cmd.Flags().GetString("data-dir"); flag != "" {
		return flag, nil
	}
	if v := os.Getenv(envDataDir); v != "" {
		return v, nil
	}
	if global, _ := cmd.Flags().GetBool("global"); global {
		return store.GlobalDataDir()
	}
	if dir, ok := store.FindProjectDir(); ok {
		return dir, nil
	}
	return "", errors.New("no data directory found: pass --data-dir, set THING_DATA_DIR, use -g for the global tree, or run inside a project (a directory with a .thing/, found by searching upward)")
}

// configDir resolves the config directory:
//
//	--config  ->  THING_CONFIG_DIR  ->  nearest .thing/ upward (unless -g)  ->  global
//
// Unlike data, config keeps a global default ($XDG_CONFIG_HOME/thing, else
// ~/.config/thing) as the final fallback rather than erroring.
func configDir(cmd *cobra.Command) (string, error) {
	if flag, _ := cmd.Flags().GetString("config"); flag != "" {
		return flag, nil
	}
	if v := os.Getenv(envConfigDir); v != "" {
		return v, nil
	}
	if global, _ := cmd.Flags().GetBool("global"); !global {
		if dir, ok := store.FindProjectDir(); ok {
			return dir, nil
		}
	}
	return store.GlobalConfigDir()
}

// initDataDir / initConfigDir resolve where `thing init` creates directories.
// Like npm init, a bare init anchors a new project at ./.thing rather than
// searching upward; -g targets the global directories.
func initDataDir(cmd *cobra.Command) (string, error) {
	if flag, _ := cmd.Flags().GetString("data-dir"); flag != "" {
		return flag, nil
	}
	if v := os.Getenv(envDataDir); v != "" {
		return v, nil
	}
	if global, _ := cmd.Flags().GetBool("global"); global {
		return store.GlobalDataDir()
	}
	return store.ProjectDir, nil
}

func initConfigDir(cmd *cobra.Command) (string, error) {
	if flag, _ := cmd.Flags().GetString("config"); flag != "" {
		return flag, nil
	}
	if v := os.Getenv(envConfigDir); v != "" {
		return v, nil
	}
	if global, _ := cmd.Flags().GetBool("global"); global {
		return store.GlobalConfigDir()
	}
	return store.ProjectDir, nil
}

// splitTrim splits s on sep, trimming whitespace from each field and dropping
// blanks. An all-blank input yields nil.
func splitTrim(s, sep string) []string {
	var out []string
	for _, p := range strings.Split(s, sep) {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
