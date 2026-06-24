// Package golden compares test output against testdata/<name>.atom snapshots,
// shared by the RSS feed golden tests. Regenerate with UPDATE_GOLDEN=1.
package golden

import (
	"os"
	"path/filepath"
	"testing"
)

// Assert compares got against testdata/<name>.atom in the calling package's
// directory (go test runs with the working directory set to the package dir).
// With UPDATE_GOLDEN=1 it (re)writes the golden instead of comparing.
func Assert(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".atom")

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (regenerate with UPDATE_GOLDEN=1)", path, err)
	}
	if got != string(want) {
		t.Fatalf("output != golden %s\n--- got ---\n%s\n--- want ---\n%s", path, got, string(want))
	}
}
