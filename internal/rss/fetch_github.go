package rss

import (
	"fmt"

	"go.uber.org/zap"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/eli-yip/rss-zero/pkg/render"
	githubDB "github.com/eli-yip/rss-zero/pkg/routers/github/db"
)

// FetchGitHub builds the canonical feed for a github release subscription, loading
// up to MaxFetch releases. It is the source's Fetch step (request cache-miss and
// cron warm).
func FetchGitHub(subID string, db githubDB.DB, logger *zap.Logger) (FeedMeta, []Item, error) {
	sub, err := db.GetSubByIDIncludeDeleted(subID)
	if err != nil {
		return FeedMeta{}, nil, fmt.Errorf("failed to get sub info from database: %w", err)
	}
	repo, err := db.GetRepoByID(sub.RepoID)
	if err != nil {
		return FeedMeta{}, nil, fmt.Errorf("failed to get repo info from database: %w", err)
	}

	releases, err := db.GetReleases(repo.ID, sub.PreRelease, 1, MaxFetch)
	if err != nil {
		return FeedMeta{}, nil, fmt.Errorf("failed to get releases from database: %w", err)
	}
	if len(releases) == 0 {
		logger.Info("found no github release, building empty feed")
	}

	return feedFromGitHubReleases(repo.GithubUser, repo.Name, sub.PreRelease, releases)
}

// feedFromGitHubReleases builds the envelope and items from already-loaded releases.
// Split out so the differential test diffs it against the former render.Render
// without a DB. The "Tag:" content prefix, repo-cased titles and pre-release
// markers are preserved; markdown→HTML goes through the shared FeedHTML (the github
// renderer already used the same goldmark config, so there is no A6 change).
func feedFromGitHubReleases(user, repoName string, pre bool, releases []githubDB.Release) (FeedMeta, []Item, error) {
	casedRepo := cases.Title(language.English, cases.Compact).String(repoName)
	feedTitle := githubFeedTitle(casedRepo, pre)
	if len(releases) == 0 {
		return FeedMeta{
			Title:   feedTitle,
			Link:    fmt.Sprintf("https://github.com/%s/%s/releases", user, repoName),
			Updated: defaultTime,
		}, nil, nil
	}

	meta := FeedMeta{
		Title:   feedTitle,
		Link:    releases[0].URL,
		Updated: releases[0].PublishedAt,
	}

	items := make([]Item, 0, len(releases))
	for i := range releases {
		r := releases[i]
		title := r.Title
		if title == "" {
			title = r.Tag
		}
		body := r.Body
		if body == "" {
			body = r.RawBody
		}
		contentHTML, err := render.FeedHTML(fmt.Sprintf("Tag: %s\n\n%s", r.Tag, body))
		if err != nil {
			return FeedMeta{}, nil, fmt.Errorf("failed to render github content: %w", err)
		}
		items = append(items, Item{
			ID:          fmt.Sprintf("%d", r.ID),
			Link:        r.URL,
			Title:       githubItemTitle(casedRepo, title, r.PreRelease),
			Author:      repoName,
			Time:        r.PublishedAt,
			Summary:     render.ExtractExcerpt(body),
			ContentHTML: contentHTML,
		})
	}
	return meta, items, nil
}

func githubFeedTitle(casedRepo string, pre bool) string {
	title := "[GitHub]" + casedRepo
	if pre {
		title += "-WithPre"
	}
	return title
}

func githubItemTitle(casedRepo, title string, pre bool) string {
	title = casedRepo + ": " + title
	if pre {
		return "[Pre-Release]" + title
	}
	return title
}
