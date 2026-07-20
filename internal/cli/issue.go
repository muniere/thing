package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/slug"
)

func newIssueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue",
		Short: "Manage issue nodes",
	}
	cmd.AddCommand(newIssueAddCmd())
	return cmd
}

func newIssueAddCmd() *cobra.Command {
	var priority, tags, epic string
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add an issue (orphan when --epic is omitted)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.TrimSpace(strings.Join(args, " "))
			if title == "" {
				return fmt.Errorf("a title is required")
			}
			if err := checkPriority(priority); err != nil {
				return err
			}
			st, err := openStore(cmd)
			if err != nil {
				return err
			}
			if epic != "" {
				loc, err := st.Locate(epic)
				if err != nil {
					return err
				}
				if loc == nil || loc.Node.Type != model.Epic {
					return fmt.Errorf("no such epic %q", epic)
				}
			}
			taken, err := st.AllSlugs()
			if err != nil {
				return err
			}
			n := &model.Node{
				Title:    title,
				Priority: model.Priority(priority),
				Tags:     splitTags(tags),
				Updated:  today(),
				Slug:     slug.Unique(slug.Slugify(title), taken),
			}
			if err := st.CreateIssue(n, epic); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), n.Slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&epic, "epic", "", "parent epic slug (orphan if omitted)")
	cmd.Flags().StringVar(&priority, "priority", "", "priority (high|medium|low)")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated tags")
	return cmd
}
