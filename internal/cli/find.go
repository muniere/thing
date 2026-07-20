package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/search"
	"github.com/muniere/thing/internal/store"
)

func newFindCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "find <query>",
		Short: "Fuzzy-search nodes by title, slug, and tags",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore(cmd)
			if err != nil {
				return err
			}
			idx, err := st.Index()
			if err != nil {
				return err
			}
			entries := make([]*store.Entry, 0, len(idx))
			for _, e := range idx {
				entries = append(entries, e)
			}
			results := search.Find(entries, strings.TrimSpace(strings.Join(args, " ")))

			out := cmd.OutOrStdout()
			if asJSON {
				if results == nil {
					results = []search.Result{}
				}
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}
			for _, r := range results {
				title := r.Title
				if title == "" {
					title = r.Slug
				}
				fmt.Fprintf(out, "%s  %s  [%s]\n", r.Ref, title, r.Type)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output a ranked JSON array")
	return cmd
}
