package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/slug"
	"github.com/muniere/thing/internal/store"
)

func newAddCmd() *cobra.Command {
	var priority, tags, category string
	cmd := &cobra.Command{
		Use:   "add <path>",
		Short: "Add a node addressed by a path [<parent>/]<title>",
		Long: "Add a node addressed by a path [<parent>/]<title>. With no parent " +
			"it is an epic; under an epic it is an issue; under an issue it is a " +
			"task; under _orphan it is an orphan issue. Prints the new slug.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := strings.TrimSpace(strings.Join(args, " "))
			if strings.HasPrefix(path, "/") {
				return fmt.Errorf("invalid path %q: an empty parent", path)
			}
			parent, title := cutPath(path)
			if title == "" {
				return fmt.Errorf("a title is required")
			}
			if priority != "" && !model.Priority(priority).Valid() {
				return fmt.Errorf("invalid priority %q (want %s)", priority, model.PriorityValues())
			}
			if category != "" && parent != "" {
				return fmt.Errorf("--category applies only to an epic (a node with no parent)")
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
				Priority: model.Priority(priority),
				Tags:     splitTrim(tags, ","),
				Updated:  today(),
				Slug:     slug.Unique(slug.Slugify(title), taken),
			}
			if err := create(st, n, parent, category); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), n.Slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "category (epics only)")
	cmd.Flags().StringVar(&priority, "priority", "", "priority (high|medium|low)")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated tags")
	return cmd
}

// create places n under parent: no parent -> epic; _orphan -> orphan issue; an
// epic -> issue; an issue -> task.
func create(st *store.Store, n *model.Node, parent, category string) error {
	switch parent {
	case "":
		n.Category = category
		return st.CreateEpic(n)
	case store.OrphanDir:
		return st.CreateIssue(n, "")
	}
	p, err := st.Get(parent)
	if err != nil {
		return err
	}
	switch p.Node.Type {
	case model.Epic:
		return st.CreateIssue(n, parent)
	case model.Issue:
		return st.CreateTask(n, p.Dir)
	default:
		return fmt.Errorf("cannot add a node under a task")
	}
}

// cutPath splits [<parent>/]<title> on its first slash; a path with no slash is
// all title (a top-level epic). The parent is a single slug, so the title keeps
// any later slashes.
func cutPath(path string) (parent, title string) {
	if before, after, found := strings.Cut(path, "/"); found {
		return before, strings.TrimSpace(after)
	}
	return "", path
}
