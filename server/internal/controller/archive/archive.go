package archive

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/utils"
	"github.com/eli-yip/rss-zero/pkg/render"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

func (h *Controller) Archive(c echo.Context) (err error) {
	type (
		Item struct {
			Title    string `json:"title"`
			Content  string `json:"content"`
			Author   string `json:"author"`
			Platform string `json:"platform"`
			Type     string `json:"type"`
			ID       string `json:"id"`
			Time     int    `json:"time"`
			Link     string `json:"link"`
		}

		Response struct {
			Items []Item `json:"items"`
		}

		ErrResponse struct {
			Message string `json:"message"`
		}
	)

	logger := common.ExtractLogger(c)

	contentType := c.QueryParam("content_type")
	author := c.QueryParam("author")
	limit := c.QueryParam("limit")
	offset := c.QueryParam("offset")

	zhihuDBService := zhihuDB.NewDBService(h.db)

	switch contentType {
	case "answer":
		limitInt, err := strconv.Atoi(limit)
		if err != nil {
			logger.Error("Failed to convert limit to int", zap.String("limit", limit))
			return c.JSON(http.StatusBadRequest, ErrResponse{Message: "Invalid limit"})
		}
		offsetInt, err := strconv.Atoi(offset)
		if err != nil {
			logger.Error("Failed to convert offset to int", zap.String("offset", offset))
			return c.JSON(http.StatusBadRequest, ErrResponse{Message: "Invalid offset"})
		}
		answers, err := zhihuDBService.FetchAnswer(author, limitInt, offsetInt)
		if err != nil {
			logger.Error("Failed to fetch answer", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, ErrResponse{Message: "Failed to fetch answer"})
		}

		var items []Item
		for _, answer := range answers {
			question, err := zhihuDBService.GetQuestion(answer.QuestionID)
			if err != nil {
				logger.Error("Failed to get question", zap.Error(err))
				return c.JSON(http.StatusInternalServerError, ErrResponse{Message: "Failed to get question"})
			}
			items = append(items, Item{
				Title:    question.Title,
				Content:  answer.Text,
				Author:   answer.AuthorID,
				Platform: "zhihu",
				Type:     "answer",
				ID:       strconv.Itoa(answer.ID),
				Link:     fmt.Sprintf("https://www.zhihu.com/question/%d/answer/%d", answer.QuestionID, answer.ID),
				Time:     int(utils.TimeToUnix(answer.CreateAt)),
			})
		}

		return c.JSON(http.StatusOK, Response{Items: items})
	case "article":
		fallthrough
	case "pin":
		fallthrough
	case "all":
		fallthrough
	default:
		logger.Info("Invalid content type", zap.String("content_type", contentType))
		return c.JSON(http.StatusBadRequest, ErrResponse{Message: "Invalid content type"})
	}
}

type archiveResult struct {
	html, redirectTo string
}

func (h *Controller) History(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	requestID := c.Response().Header().Get(echo.HeaderXRequestID)

	u := c.Param("url")
	u, err = url.PathUnescape(u)
	if err != nil {
		logger.Error("Failed to unescape url", zap.Error(err))
		return c.HTML(http.StatusBadRequest, renderErrorPage(err, requestID))
	}
	logger.Info("Get history url", zap.String("url", u))

	params := c.QueryParams()
	if len(params) > 0 {
		return c.Redirect(http.StatusFound, render.BuildArchiveLink(config.C.Settings.ServerURL, u))
	}

	result, err := h.handleRequestArchiveLink(u)
	if err != nil {
		logger.Error("Failed to get webarchive", zap.Error(err))
		return c.HTML(http.StatusBadRequest, renderErrorPage(err, requestID))
	}
	if result.redirectTo != "" {
		return c.Redirect(http.StatusFound, result.redirectTo)
	}
	return c.HTML(http.StatusOK, result.html)
}

func (h *Controller) handleRequestArchiveLink(link string) (result *archiveResult, err error) {
	switch {
	case regexp.MustCompile(`/question/\d+/answer/\d+`).MatchString(link):
		return h.HandleZhihuAnswer(link)
	case regexp.MustCompile(`/p/\d+`).MatchString(link):
		return h.HandleZhihuArticle(link)
	case regexp.MustCompile(`/pin/\d+`).MatchString(link):
		return h.HandleZhihuPin(link)
	case regexp.MustCompile(`/topic_detail/\d+`).MatchString(link):
		return h.HandleZsxqWebTopic(link)
	case regexp.MustCompile(`/group/\d+/topic/\d+`).MatchString(link):
		return h.HandleZsxqWebTopic(link)
	case regexp.MustCompile(`t\.zsxq\.com/\w+`).MatchString(link):
		return h.HandleZsxqShareLink(link)
	}
	return nil, fmt.Errorf("unknown link: %s", link)
}

func renderErrorPage(err error, requestID string) string {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
		<!-- Standard favicon -->
		<link rel="icon" href="https://oss.darkeli.com/rss/favicon/favicon.ico" type="image/x-icon">

		<!-- 16x16 icon -->
		<link rel="icon" href="https://oss.darkeli.com/rss/favicon/favicon-16x16.png" sizes="16x16" type="image/png">

		<!-- 32x32 icon -->
		<link rel="icon" href="https://oss.darkeli.com/rss/favicon/favicon-32x32.png" sizes="32x32" type="image/png">

		<!-- Android Chrome icon -->
		<link rel="icon" href="https://oss.darkeli.com/rss/favicon/android-chrome-192x192.png" sizes="192x192" type="image/png">
		<link rel="icon" href="https://oss.darkeli.com/rss/favicon/android-chrome-512x512.png" sizes="512x512" type="image/png">

		<!-- Apple Touch icon for iOS -->
		<link rel="apple-touch-icon" href="https://oss.darkeli.com/rss/favicon/apple-touch-icon.png">

		<!-- Web App Manifest -->
		<link rel="manifest" href="https://oss.darkeli.com/rss/favicon/site.webmanifest">
    <title>Archive History Error</title>
    <style>
        body {
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            font-family: Arial, sans-serif;
            background-color: #f8f9fa;
        }
        .error-box {
            width: 600px;
            height: 150px;
            background-color: #a1afc9;
            color: black;
            display: flex;
            justify-content: center;
            align-items: center;
            text-align: center;
            padding: 20px;
            box-shadow: 0 4px 8px rgba(0, 0, 0, 0.2);
            border-radius: 8px;
						flex-direction: column;
        }
    </style>
</head>
<body>
    <div class="error-box">
        <h2>%s</h2>
				<p><pre><code>%s</code></pre></p>
    </div>
</body>
</html>`
	return fmt.Sprintf(tmpl, err.Error(), requestID)
}
