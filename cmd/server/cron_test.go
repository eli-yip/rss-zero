package main

import (
	"slices"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestRandomDelayRunnerWrap(t *testing.T) {
	t.Parallel()

	const (
		maxDelay = 10 * time.Minute
		delay    = 3 * time.Minute
	)
	events := make([]string, 0, 3)
	runner := randomDelayRunner{
		maxDelay: maxDelay,
		random: func(gotMax time.Duration) time.Duration {
			if gotMax != maxDelay {
				t.Fatalf("max delay = %s, want %s", gotMax, maxDelay)
			}
			events = append(events, "random")
			return delay
		},
		sleep: func(gotDelay time.Duration) {
			if gotDelay != delay {
				t.Fatalf("sleep delay = %s, want %s", gotDelay, delay)
			}
			events = append(events, "sleep")
		},
	}

	runner.wrap("test_crawl", zap.NewNop(), func() { events = append(events, "run") })()

	want := []string{"random", "sleep", "run"}
	if !slices.Equal(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestBuildStaticJobDefinitionsRandomDelay(t *testing.T) {
	t.Parallel()

	wantDelayed := []string{"douyu_crawl", "macked_crawl", "tombkeeper_crawl", "zvideo_crawl"}
	wantSchedules := map[string]string{
		"douyu_crawl":      "0 19 * * *",
		"macked_crawl":     "0 * * * *",
		"tombkeeper_crawl": "0 * * * *",
		"zvideo_crawl":     "0 0,3,6,9,12,15,18,21 * * *",
	}
	jobs := buildStaticJobDefinitions(nil, nil, nil, nil, nil, nil, false)
	gotDelayed := make([]string, 0, len(wantDelayed))
	for _, job := range jobs {
		if job.randomDelay {
			gotDelayed = append(gotDelayed, job.name)
			if job.schedule != wantSchedules[job.name] {
				t.Errorf("%s schedule = %q, want %q", job.name, job.schedule, wantSchedules[job.name])
			}
		}
	}
	slices.Sort(gotDelayed)

	if !slices.Equal(gotDelayed, wantDelayed) {
		t.Fatalf("randomly delayed jobs = %v, want %v", gotDelayed, wantDelayed)
	}
}

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
