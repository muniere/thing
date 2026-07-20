package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/slug"
)

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage task nodes",
	}
	cmd.AddCommand(newTaskAddCmd())
	return cmd
}

func newTaskAddCmd() *cobra.Command {
	var priority, tags, issue string
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a task under an issue",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.TrimSpace(strings.Join(args, " "))
			if title == "" {
				return fmt.Errorf("a title is required")
			}
			if issue == "" {
				return fmt.Errorf("--issue is required")
			}
			if err := checkPriority(priority); err != nil {
				return err
			}
			st, err := openStore(cmd)
			if err != nil {
				return err
			}
			loc, err := st.Locate(issue)
			if err != nil {
				return err
			}
			if loc == nil || loc.Node.Type != model.Issue {
				return fmt.Errorf("no such issue %q", issue)
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
			if err := st.CreateTask(n, loc.Dir); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), n.Slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "parent issue slug (required)")
	cmd.Flags().StringVar(&priority, "priority", "", "priority (high|medium|low)")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated tags")
	return cmd
}
