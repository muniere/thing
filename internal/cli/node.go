package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/muniere/thing/internal/model"
	"github.com/muniere/thing/internal/store"
)

// checkPriority validates an optional priority flag value.
func checkPriority(p string) error {
	if p != "" && !model.Priority(p).Valid() {
		return fmt.Errorf("invalid priority %q", p)
	}
	return nil
}

// splitTags parses a comma-separated tag list, dropping blanks.
func splitTags(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func today() string {
	return time.Now().Format("2006-01-02")
}

// locate resolves slug and verifies it is of the expected type.
func locate(cmd *cobra.Command, res model.NodeType, slug string) (*store.Located, error) {
	st, err := openStore(cmd)
	if err != nil {
		return nil, err
	}
	loc, err := st.Locate(slug)
	if err != nil {
		return nil, err
	}
	if loc == nil || loc.Node.Type != res {
		return nil, fmt.Errorf("no such %s %q", res, slug)
	}
	return loc, nil
}
