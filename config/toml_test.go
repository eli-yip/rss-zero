package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitFromTomlDisableDouyu(t *testing.T) {
	original := C
	t.Cleanup(func() { C = original })

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[settings]\ndisable_douyu = true\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := InitFromToml(path); err != nil {
		t.Fatalf("InitFromToml: %v", err)
	}
	if !C.Settings.DisableDouyu {
		t.Fatal("DisableDouyu = false, want true")
	}
}
