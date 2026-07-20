package main

import (
	"slices"
	"testing"
)

func TestBuildStaticJobDefinitionsDisableDouyu(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		disableDouyu bool
		wantDouyu    bool
	}{
		{name: "enabled by default", wantDouyu: true},
		{name: "disabled", disableDouyu: true, wantDouyu: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			jobs := buildStaticJobDefinitions(nil, nil, nil, nil, nil, nil, tt.disableDouyu)
			names := make([]string, 0, len(jobs))
			for _, job := range jobs {
				names = append(names, job.name)
			}

			if got := slices.Contains(names, "douyu_crawl"); got != tt.wantDouyu {
				t.Fatalf("douyu_crawl present = %t, want %t; jobs = %v", got, tt.wantDouyu, names)
			}
			if !slices.Contains(names, "check_cookies") {
				t.Fatalf("unrelated static job missing; jobs = %v", names)
			}
		})
	}
}
