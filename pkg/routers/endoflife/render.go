package endoflife

import (
	"bytes"
	"fmt"

	"github.com/gorilla/feeds"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type RSSRenderer interface {
	RenderRSS(product string, versionInfoList []versionInfo) (content string, err error)
}

type RSSRenderService struct {
	HTMLRender goldmark.Markdown
	caser      cases.Caser
}

func NewRSSRenderService() RSSRenderer {
	return &RSSRenderService{HTMLRender: goldmark.New(
		goldmark.WithExtensions(extension.GFM,
			extension.NewCJK(extension.WithEastAsianLineBreaks(extension.EastAsianLineBreaksCSS3Draft))),
	),
		caser: cases.Title(language.English, cases.NoLower)}
}

func (r *RSSRenderService) title(str string) string {
	return r.caser.String(str)
}

func (r *RSSRenderService) RenderRSS(product string, versionInfoList []versionInfo) (content string, err error) {
	rssFeed := &feeds.Feed{
		Title:   r.title(fmt.Sprintf("%s Release", product)),
		Link:    &feeds.Link{Href: fmt.Sprintf("https://endoflife.date/%s", product)},
		Created: versionInfoList[0].releaseDate,
		Updated: versionInfoList[0].releaseDate,
	}

	for _, v := range versionInfoList {
		text := fmt.Sprintf("Version **%s** of **%s** was released on %s.\n\nBranch: %s", versionToVersionString(v.version), product, v.releaseDate.Format("2006-01-02"),
			func() string {
				if v.lts {
					return "**LTS**"
				}
				return "**Latest**"
			}(),
		)

		var buffer bytes.Buffer
		if err := r.HTMLRender.Convert([]byte(text), &buffer); err != nil {
			return "", err
		}

		feedItem := feeds.Item{
			Title: func() string {
				var title string
				if v.lts {
					title = r.title(fmt.Sprintf("%s LTS %s released", product, versionToVersionString(v.version)))
				} else {
					title = r.title(fmt.Sprintf("%s Latest %s released", product, versionToVersionString(v.version)))
				}
				return title
			}(),
			Link:        &feeds.Link{Href: fmt.Sprintf("https://endoflife.date/%s", product)},
			Author:      &feeds.Author{Name: "EndOfLife"},
			Id:          fmt.Sprintf("%s-%s", product, versionToVersionString(v.version)),
			Description: text,
			Created:     v.releaseDate,
			Updated:     v.releaseDate,
			Content:     buffer.String(),
		}

		rssFeed.Items = append(rssFeed.Items, &feedItem)
	}

	return rssFeed.ToAtom()
}
