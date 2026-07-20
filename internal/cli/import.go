package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/importer"
)

func newImportCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Bulk-create nodes from a JSON batch",
		Long: "Bulk-create nodes from a flat JSON array of {type,title,parent," +
			"priority,category,tags,links,body}. type defaults to task; parent is a " +
			"ref, or \"inbox\" for a task, or empty for an epic / orphan issue. The " +
			"batch is flat (no children), so an `export` file is not an import file. " +
			"Prints a JSON result array (one entry per item, in order) and exits " +
			"non-zero if any item failed. With --dry-run, parents are validated and " +
			"refs assigned (status \"validated\") but nothing is written.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			st, err := openStore(cmd)
			if err != nil {
				return err
			}
			results, ok, err := importer.Run(st, data, dryRun, today())
			if err != nil {
				return err
			}
			out, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			if !ok {
				return errors.New("import completed with errors")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate and assign refs without writing")
	return cmd
}
