package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/config"
	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/render"
	"github.com/muniere/thing/internal/store"
)

func newLsCmd() *cobra.Command {
	var archived, all bool
	cmd := &cobra.Command{
		Use:   "ls [<ref>]",
		Short: "List a node's children, or the top level when omitted",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(cmd)
			if err != nil {
				return err
			}

			// The archive is a hidden top-level region, so its flags do not descend
			// into a ref.
			if (archived || all) && len(args) > 0 {
				return fmt.Errorf("--archived/--all list the top level; they take no <ref>")
			}

			// --archived shows only the archive; --all appends it after the tree.
			if archived {
				out, err := archiveListing(st)
				if err != nil {
					return err
				}
				fmt.Fprint(cmd.OutOrStdout(), out)
				return nil
			}

			// The top level (epics and orphan issues) is grouped under the
			// configured category headings.
			if len(args) == 0 {
				top, err := st.Load()
				if err != nil {
					return err
				}
				cfgDir, err := configDir(cmd)
				if err != nil {
					return err
				}
				cfg, err := config.Load(cfgDir)
				if err != nil {
					return err
				}
				fmt.Fprint(cmd.OutOrStdout(), render.TopList(top, cfg.Categories))
				if all {
					out, err := archiveListing(st)
					if err != nil {
						return err
					}
					if out != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "\n%s", out)
					}
				}
				return nil
			}

			var nodes []*model.Node
			if args[0] == store.OrphanDir {
				top, err := st.Load()
				if err != nil {
					return err
				}
				for _, n := range top {
					if n.Type == model.Issue {
						nodes = append(nodes, n)
					}
				}
			} else {
				loc, err := st.Get(args[0])
				if err != nil {
					return err
				}
				nodes = loc.Node.Children
			}
			fmt.Fprint(cmd.OutOrStdout(), render.List(nodes))
			return nil
		},
	}
	cmd.Flags().BoolVar(&archived, "archived", false, "list only archived entries")
	cmd.Flags().BoolVar(&all, "all", false, "list the top level plus archived entries")
	cmd.MarkFlagsMutuallyExclusive("archived", "all")
	return cmd
}

// archiveListing renders the hidden archive region: each entry's archive ref, the
// ref it was archived from, its title, and the archive time.
func archiveListing(st *store.Store) (string, error) {
	entries, err := st.ArchiveList()
	if err != nil {
		return "", err
	}
	items := make([]render.ArchiveItem, len(entries))
	for i, e := range entries {
		items[i] = render.ArchiveItem{
			Ref:        e.Ref,
			From:       e.Node.ArchivedRef,
			Title:      e.Node.Title,
			ArchivedAt: e.Node.ArchivedAt,
		}
	}
	return render.ArchiveList(items), nil
}
