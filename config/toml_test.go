package config

import (
	"os"
	"path/filepath"
	"slices"
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

func TestInitFromTomlWeiboGroups(t *testing.T) {
	original := C
	t.Cleanup(func() { C = original })

	for _, tc := range []struct {
		name string
		toml string
		want []string
	}{
		{name: "absent"},
		{name: "empty", toml: "[weibo]\ngroup_ids = []\n"},
		{
			name: "configured",
			toml: "[weibo]\ngroup_ids = ['9007199254740993', '9007199254740995']\n",
			want: []string{"9007199254740993", "9007199254740995"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			C = TomlConfig{}
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(tc.toml), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			if err := InitFromToml(path); err != nil {
				t.Fatalf("InitFromToml: %v", err)
			}
			if !slices.Equal(C.Weibo.GroupIDs, tc.want) {
				t.Fatalf("GroupIDs = %v, want %v", C.Weibo.GroupIDs, tc.want)
			}
		})
	}
}
