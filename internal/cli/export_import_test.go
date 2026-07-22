package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportImportRoundtrip(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}

	batch := `[
		{"type":"epic","title":"Infra"},
		{"type":"issue","title":"Provision","parent":"infra"},
		{"type":"task","title":"Apply","parent":"infra/provision"},
		{"title":"Loose note","parent":"inbox"}
	]`
	file := filepath.Join(dir, "batch.json")
	if err := os.WriteFile(file, []byte(batch), 0o644); err != nil {
		t.Fatal(err)
	}

	// import writes the batch and prints a JSON result array.
	code, out, errb := runCLI(t, append([]string{"import", file}, d...)...)
	if code != 0 {
		t.Fatalf("import: code=%d err=%s", code, errb)
	}
	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("import output not JSON: %v\n%s", err, out)
	}
	if len(results) != 4 || results[0]["ref"] != "infra" {
		t.Fatalf("unexpected results: %s", out)
	}

	// export reflects what import created, nested.
	code, out, errb = runCLI(t, append([]string{"export"}, d...)...)
	if code != 0 {
		t.Fatalf("export: code=%d err=%s", code, errb)
	}
	if !strings.Contains(out, `"ref": "infra"`) || !strings.Contains(out, `"ref": "infra/provision/apply"`) {
		t.Fatalf("export missing imported nodes: %s", out)
	}
}

func TestImportDryRunExitsNonZeroOnError(t *testing.T) {
	dir := t.TempDir()
	d := []string{"--data-dir", dir, "--config", dir}

	batch := `[{"type":"task","title":"Orphaned","parent":"ghost"}]`
	file := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(file, []byte(batch), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out, _ := runCLI(t, append([]string{"import", "--dry-run", file}, d...)...)
	if code == 0 {
		t.Fatalf("expected non-zero exit on item error")
	}
	if !strings.Contains(out, `"status": "error"`) {
		t.Fatalf("expected error status in output: %s", out)
	}
	// Dry-run must not have written anything: the tree still exports as empty.
	code, out, _ = runCLI(t, append([]string{"export"}, d...)...)
	if code != 0 || strings.TrimSpace(out) != "[]" {
		t.Fatalf("dry-run should have written nothing, export=%q", out)
	}
}
