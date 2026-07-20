package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/render"
	"github.com/muniere/thing/internal/slug"
)

func newEpicCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "epic",
		Short: "Manage epic nodes",
	}
	cmd.AddCommand(
		newEpicAddCmd(),
		newEpicListCmd(),
		newEpicShowCmd(),
		newEpicStatusCmd(),
		newEpicPriorityCmd(),
	)
	return cmd
}

func newEpicStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <slug> <status>",
		Short: "Set an epic's status",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(cmd)
			if err != nil {
				return err
			}
			loc, err := st.Find(args[0], model.Epic)
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

func newEpicPriorityCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "priority <slug> <priority>",
		Short: "Set an epic's priority",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(cmd)
			if err != nil {
				return err
			}
			loc, err := st.Find(args[0], model.Epic)
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

func newEpicShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <slug>",
		Short: "Show an epic and its body",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(cmd)
			if err != nil {
				return err
			}
			loc, err := st.Find(args[0], model.Epic)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), render.Show(loc.Node))
			return nil
		},
	}
}

func newEpicListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List epics",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			st, err := openStore(cmd)
			if err != nil {
				return err
			}
			nodes, err := st.Epics()
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), render.List(nodes))
			return nil
		},
	}
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
			if priority != "" && !model.Priority(priority).Valid() {
				return fmt.Errorf("invalid priority %q", priority)
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
				Tags:     splitTrim(tags, ","),
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
