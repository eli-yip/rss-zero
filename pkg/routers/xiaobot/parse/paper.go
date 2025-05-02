package parse

import (
	"encoding/json"

	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/parse/api_models"
	"go.uber.org/zap"
)

func (p *ParseService) SplitPaper(data json.RawMessage) (posts []apiModels.PaperPost, err error) {
	posts = make([]apiModels.PaperPost, 0)
	if err = json.Unmarshal(data, &posts); err != nil {
		return nil, err
	}
	return posts, nil
}

func (p *ParseService) ParsePaper(data []byte, logger *zap.Logger) (paperName string, err error) {
	var paper apiModels.Paper
	if err = json.Unmarshal(data, &paper); err != nil {
		return "", err
	}
	logger.Info("Unmarshal data to paper")

	if err = p.db.SaveCreator(&db.Creator{
		ID:       paper.Creator.ID,
		NickName: paper.Creator.NickName,
	}); err != nil {
		return "", err
	}
	logger.Info("Save creator to db", zap.String("id", paper.Creator.NickName))

	if err = p.db.SavePaper(&db.Paper{
		ID:        paper.Slug,
		Name:      paper.Name,
		CreatorID: paper.Creator.ID,
		Intro:     paper.Intro,
	}); err != nil {
		return "", err
	}
	logger.Info("Save paper to db", zap.String("name", paper.Name))

	return paper.Name, nil
}

func (p *ParseService) ParsePaperPost(data []byte, paperID string, logger *zap.Logger) (text string, err error) {
	var post apiModels.PaperPost
	if err = json.Unmarshal(data, &post); err != nil {
		return "", err
	}
	logger = logger.With(zap.String("post_id", post.ID))
	logger.Info("Unmarshal data to post")

	// if the post is a description, we don't need to parse it
	if post.IsDescription == 1 {
		logger.Info("Skip parsing description post")
		return "", nil
	}

	textBytes, err := p.Convert([]byte(post.HTML))
	if err != nil {
		return "", err
	}
	logger.Info("Convert HTML to markdown")

	text, err = p.FormatStr(string(textBytes))
	if err != nil {
		return "", err
	}
	logger.Info("Format markdown")

	t, err := p.ParseTime(post.CreateAt)
	if err != nil {
		return "", err
	}

	if err = p.db.SavePost(&db.Post{
		ID:       post.ID,
		PaperID:  paperID,
		CreateAt: t,
		Title:    post.Title,
		Text:     text,
		Raw:      data,
	}); err != nil {
		return "", err
	}
	logger.Info("Save post to db successfully")

	return text, nil
}
