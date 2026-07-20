package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/model"
)

func newAddCmd() *cobra.Command {
	var priority, tags, category string
	cmd := &cobra.Command{
		Use:   "add [<parent>/]<title>",
		Short: "Add a node addressed by a path [<parent>/]<title>",
		Long: "Add a node addressed by a path [<parent>/]<title>. With no parent " +
			"it is an epic; under an epic it is an issue; under an issue it is a " +
			"task; under _orphan it is an orphan issue. Prints the new node's path.",
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
			n := &model.Node{
				Title:    title,
				Category: category,
				Priority: model.Priority(priority),
				Tags:     splitTrim(tags, ","),
				Updated:  today(),
			}
			newRef, err := st.Add(parent, n)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), newRef)
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "category (epics only)")
	cmd.Flags().StringVar(&priority, "priority", "", "priority (high|medium|low)")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated tags")
	return cmd
}

// cutPath splits [<parent>/]<title> on its last slash: the parent is a slug
// path (possibly multi-level, e.g. "epic/issue"), the title is the final
// segment. A path with no slash is all title (a top-level epic).
func cutPath(path string) (parent, title string) {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i], strings.TrimSpace(path[i+1:])
	}
	return "", path
}
