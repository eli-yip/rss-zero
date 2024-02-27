package rss

import (
	"fmt"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/redis"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	render "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/render"
	"go.uber.org/zap"
)

func GenerateXiaobot(paperID string, d xiaobotDB.DB, l *zap.Logger) (path string, result string, err error) {
	l.Info("Start to generate xiaobot rss")
	rssRender := render.NewRSSRenderService()

	paper, err := d.GetPaper(paperID)
	if err != nil {
		return "", "", err
	}
	l = l.With(zap.String("paper_name", paper.Name))
	l.Info("Got paper name")

	authorName, err := d.GetCreatorName(paper.CreatorID)
	if err != nil {
		return "", "", err
	}
	l.Info("Got author name", zap.String("author_name", authorName))

	path = fmt.Sprintf(redis.XiaobotRSSPath, paperID)

	posts, err := d.GetLatestNPost(paperID, config.DefaultFetchCount)
	if err != nil {
		return "", "", err
	}
	if len(posts) == 0 {
		result, err = rssRender.RenderEmpty(paperID, paper.Name)
		l.Info("No post found, render empty rss")
		return path, result, err
	}

	rs := make([]render.RSS, 0, len(posts))
	for _, p := range posts {
		rs = append(rs, render.RSS{
			ID:         p.ID,
			Link:       fmt.Sprintf("https://xiaobot.net/post/%s", p.ID),
			CreateTime: p.CreateAt,
			PaperID:    paperID,
			PaperName:  paper.Name,
			AuthorName: authorName,
			Title:      p.Title,
			Text:       p.Text,
		})
	}

	output, err := rssRender.Render(rs)

	return path, output, err
}
