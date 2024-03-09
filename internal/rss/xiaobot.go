package rss

import (
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/redis"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	render "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/render"
	"go.uber.org/zap"
)

func GenerateXiaobot(paperID string, dbService xiaobotDB.DB, logger *zap.Logger) (path string, result string, err error) {
	logger.Info("Start to generate xiaobot rss")
	rssRender := render.NewRSSRenderService()
	logger.Info("Init xiaobot rss render service")

	paper, err := dbService.GetPaper(paperID)
	if err != nil {
		logger.Error("Fail to get paper info from database", zap.Error(err), zap.String("paper id", paperID))
		return "", "", err
	}
	logger = logger.With(zap.String("paper_name", paper.Name))
	logger.Info("Got paper name")

	authorName, err := dbService.GetCreatorName(paper.CreatorID)
	if err != nil {
		logger.Error("Fail to get author name from database", zap.Error(err), zap.String("creator id", paper.CreatorID))
		return "", "", err
	}
	logger.Info("Got author name", zap.String("author_name", authorName))

	path = fmt.Sprintf(redis.XiaobotRSSPath, paperID)

	posts, err := dbService.FetchNPostBefore(config.DefaultFetchCount, paperID, time.Now().Add(1*time.Hour)) // add 1 hour to avoid the post created at the same time with the rss generated time
	if err != nil {
		logger.Info("Fail to get xiaobot posts from database", zap.Error(err))
		return "", "", err
	}
	if len(posts) == 0 {
		result, err = rssRender.RenderEmpty(paperID, paper.Name)
		logger.Info("No post found, render empty rss")
		return path, result, err
	}
	logger.Info("Get posts from database", zap.Int("posts count", len(posts)))

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
	logger.Info("Generate xiaobot rss raw slices", zap.Int("Slice length", len(rs)))

	output, err := rssRender.Render(rs)
	if err != nil {
		logger.Error("Generate xiaobot rss output", zap.Error(err))
		return "", "", nil
	}

	return path, output, nil
}
