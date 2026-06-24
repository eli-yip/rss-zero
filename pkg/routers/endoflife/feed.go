package endoflife

import (
	"fmt"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/render"
)

// BuildFeed crawls endoflife.date for the product and builds the canonical feed
// for the unified RSS pipeline. It is the source's Fetch step: no DB, the live
// crawl is the data source.
func BuildFeed(product string) (rss.FeedMeta, []rss.Item, error) {
	cycles, err := GetReleaseCycles(product)
	if err != nil {
		return rss.FeedMeta{}, nil, fmt.Errorf("failed to get release cycles from endoflife: %w", err)
	}

	versionInfoList, err := ParseCycles(cycles)
	if err != nil {
		return rss.FeedMeta{}, nil, fmt.Errorf("failed to parse release cycles: %w", err)
	}

	// Cap to MaxFetch like every other source so the cached feed never exceeds it
	// (the list is newest-version first, so this keeps the latest). Without the cap
	// an explicit ?limit>MaxFetch would return fewer items than the default.
	if len(versionInfoList) > rss.MaxFetch {
		versionInfoList = versionInfoList[:rss.MaxFetch]
	}

	return feedFromVersions(product, versionInfoList)
}

// feedFromVersions builds the envelope and items from an already-parsed version
// list. Split out from BuildFeed so it is testable without the live crawl, and so
// the differential test can diff it against the former RenderRSS. Decoration
// matches RenderRSS byte-for-byte: a per-version markdown blurb rendered via the
// shared feed renderer, with the markdown source kept as the entry summary.
func feedFromVersions(product string, versionInfoList []versionInfo) (rss.FeedMeta, []rss.Item, error) {
	if len(versionInfoList) == 0 {
		return rss.FeedMeta{}, nil, fmt.Errorf("no version info in the list")
	}

	caser := cases.Title(language.English, cases.NoLower)
	link := fmt.Sprintf("https://endoflife.date/%s", product)

	meta := rss.FeedMeta{
		Title:   caser.String(fmt.Sprintf("%s Release", product)),
		Link:    link,
		Updated: versionInfoList[0].releaseDate,
	}

	items := make([]rss.Item, 0, len(versionInfoList))
	for _, v := range versionInfoList {
		branch := "**Latest**"
		if v.lts {
			branch = "**LTS**"
		}
		text := fmt.Sprintf("Version **%s** of **%s** was released on %s.\n\nBranch: %s",
			versionToVersionString(v.version), product, v.releaseDate.Format("2006-01-02"), branch)

		contentHTML, err := render.FeedHTML(text)
		if err != nil {
			return rss.FeedMeta{}, nil, fmt.Errorf("failed to render endoflife content: %w", err)
		}

		kind := "Latest"
		if v.lts {
			kind = "LTS"
		}
		title := caser.String(fmt.Sprintf("%s %s %s released", product, kind, versionToVersionString(v.version)))

		items = append(items, rss.Item{
			ID:          fmt.Sprintf("%s-%s", product, versionToVersionString(v.version)),
			Link:        link,
			Title:       title,
			Author:      "EndOfLife",
			Time:        v.releaseDate,
			Summary:     text,
			ContentHTML: contentHTML,
		})
	}

	return meta, items, nil
}
