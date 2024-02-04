package controller

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/eli-yip/rss-zero/internal/redis"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zhihuRender "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func (h *ZhihuController) AnswerRSS(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	authorID := c.Param("id")

	const rssPath = "zhihu_rss_answer_%s"

	rss, err := h.getRSSContent(fmt.Sprintf(rssPath, authorID), logger)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to get rss from redis")
	}
	logger.Info("rss content retrieved")

	return c.String(http.StatusOK, rss)
}

func (h *ZhihuController) ArticleRSS(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	authorID := c.Param("id")

	const rssPath = "zhihu_rss_article_%s"

	rss, err := h.getRSSContent(fmt.Sprintf(rssPath, authorID), logger)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to get rss from redis")
	}

	return c.String(http.StatusOK, rss)
}

func (h *ZhihuController) PinRSS(c echo.Context) (err error) {
	logger := c.Get("logger").(*zap.Logger)

	authorID := c.Param("id")

	const rssPath = "zhihu_rss_pin_%s"

	rss, err := h.getRSSContent(fmt.Sprintf(rssPath, authorID), logger)
	if err != nil {
		return c.String(http.StatusInternalServerError, "failed to get rss from redis")
	}

	return c.String(http.StatusOK, rss)
}

func (h *ZhihuController) getRSSContent(key string, logger *zap.Logger) (content string, err error) {
	task := task{textCh: make(chan string), errCh: make(chan error)}
	defer close(task.textCh)
	defer close(task.errCh)
	defer logger.Info("task channel closed")

	h.taskCh <- task
	task.textCh <- key
	logger.Info("task sent to task channel", zap.String("key", key))

	select {
	case content := <-task.textCh:
		return content, nil
	case err := <-task.errCh:
		return "", err
	}
}

func (h *ZhihuController) processTask() {
	for {
		task := <-h.taskCh
		key := <-task.textCh
		content, err := h.redis.Get(key)
		if err != nil {
			if err == redis.ErrKeyNotExist {
				content, err = h.generateRSS(key)
				if err != nil {
					task.errCh <- err
					continue
				}
				task.textCh <- content
				continue
			} else {
				task.errCh <- err
				continue
			}
		}
		task.textCh <- content
	}
}

func (h *ZhihuController) generateRSS(key string) (output string, err error) {
	t, authorID, err := h.extractTypeAuthorFromKey(key)
	if err != nil {
		return "", err
	}

	const defaultFetchCount = 20
	zhihuDB := zhihuDB.NewDBService(h.db)

	rssRender := zhihuRender.NewRSSRenderService()

	var rs []zhihuRender.RSS
	switch t {
	case zhihuRender.TypeAnswer:
		answers, err := zhihuDB.GetLatestNAnswer(defaultFetchCount, authorID)
		if err != nil {
			return "", err
		}

		if len(answers) == 0 {
			return "", fmt.Errorf("no answer found for author: %s", authorID)
		}

		authorName, err := zhihuDB.GetAuthorName(answers[0].AuthorID)
		if err != nil {
			return "", err
		}

		for _, a := range answers {
			question, err := zhihuDB.GetQuestion(a.QuestionID)
			if err != nil {
				return "", err
			}

			rs = append(rs, zhihuRender.RSS{
				ID:         a.ID,
				Link:       fmt.Sprintf("https://www.zhihu.com/question/%d/answer/%d", a.QuestionID, a.ID),
				CreateTime: a.CreateAt,
				AuthorID:   a.AuthorID,
				AuthorName: authorName,
				Title:      question.Title,
				Text:       a.Text,
			})
		}

		output, err = rssRender.Render(zhihuRender.TypeAnswer, rs)
		if err != nil {
			return "", err
		}
	case zhihuRender.TypeArticle:
		articles, err := zhihuDB.GetLatestNArticle(defaultFetchCount, authorID)
		if err != nil {
			return "", err
		}

		if len(articles) == 0 {
			return "", fmt.Errorf("no article found for author: %s", authorID)
		}

		authorName, err := zhihuDB.GetAuthorName(articles[0].AuthorID)
		if err != nil {
			return "", err
		}

		for _, a := range articles {
			rs = append(rs, zhihuRender.RSS{
				ID:         a.ID,
				Link:       fmt.Sprintf("https://zhuanlan.zhihu.com/p/%d", a.ID),
				CreateTime: a.CreateAt,
				AuthorID:   a.AuthorID,
				AuthorName: authorName,
				Title:      a.Title,
				Text:       a.Text,
			})
		}

		output, err = rssRender.Render(zhihuRender.TypeArticle, rs)
		if err != nil {
			return "", err
		}
	case zhihuRender.TypePin:
		pins, err := zhihuDB.GetLatestNPin(defaultFetchCount, authorID)
		if err != nil {
			return "", err
		}

		if len(pins) == 0 {
			return "", fmt.Errorf("no pin found for author: %s", authorID)
		}

		authorName, err := zhihuDB.GetAuthorName(pins[0].AuthorID)
		if err != nil {
			return "", err
		}

		for _, p := range pins {
			rs = append(rs, zhihuRender.RSS{
				ID:         p.ID,
				Link:       fmt.Sprintf("https://www.zhihu.com/pin/%d", p.ID),
				CreateTime: p.CreateAt,
				AuthorID:   p.AuthorID,
				AuthorName: authorName,
				Title:      func() string { return strconv.Itoa(p.ID) }(),
				Text:       p.Text,
			})
		}

		output, err = rssRender.Render(zhihuRender.TypePin, rs)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("invalid type: %d", t)
	}

	const rssTTL = time.Hour * 2
	if err := h.redis.Set(key, output, rssTTL); err != nil {
		return "", err
	}

	return output, nil
}

func (h *ZhihuController) extractTypeAuthorFromKey(key string) (t int, authorID string, err error) {
	const regex = `zhihu_rss_([^_]+)_([^_]+)$`
	re := regexp.MustCompile(regex)

	matches := re.FindStringSubmatch(key)

	if len(matches) != 3 {
		return 0, "", fmt.Errorf("invalid key: %s", key)
	}

	switch matches[1] {
	case "answer":
		t = zhihuRender.TypeAnswer
	case "article":
		t = zhihuRender.TypeArticle
	case "pin":
		t = zhihuRender.TypePin
	default:
		return 0, "", fmt.Errorf("invalid type: %s", matches[1])
	}

	authorID = matches[2]

	return t, authorID, nil
}
