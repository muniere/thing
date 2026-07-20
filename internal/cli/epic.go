package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/slug"
)

func newEpicCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epic",
		Short: "Manage epic nodes",
	}
	cmd.AddCommand(newEpicAddCmd())
	return cmd
}

func newEpicAddCmd() *cobra.Command {
	var priority, tags, category string
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add an epic",
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
			taken, err := st.AllSlugs()
			if err != nil {
				return err
			}
			n := &model.Node{
				Title:    title,
				Category: category,
				Priority: model.Priority(priority),
				Tags:     splitTags(tags),
				Updated:  today(),
				Slug:     slug.Unique(slug.Slugify(title), taken),
			}
			if err := st.CreateEpic(n); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), n.Slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "category")
	cmd.Flags().StringVar(&priority, "priority", "", "priority (high|medium|low)")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated tags")
	return cmd
}
