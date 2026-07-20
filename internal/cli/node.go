package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/muniere/thing/internal/model"
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
