package archive

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/controller/common"
	utils "github.com/eli-yip/rss-zero/internal/utils"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/eli-yip/rss-zero/pkg/render"
	tkblog "github.com/eli-yip/rss-zero/pkg/routers/tkblog"
	tk "github.com/eli-yip/rss-zero/pkg/routers/tombkeeper"
)

// POST /api/v1/archive
func (h *Controller) Archive(c *echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req ArchiveRequest
	if err = c.Bind(&req); err != nil {
		logger.Error("Failed to bind request", zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, "Invalid request")
	}
	logger.Info("Retrieved archive request successfully")

	supportedAuthors := []string{"canglimo", "zi-e-79-23", "fu-lan-ke-yang", "ffancage"}
	if req.Platform != PlatformZhihu ||
		!slices.Contains(supportedAuthors, req.Author) {
		logger.Error("Invalid request parameters", zap.Any("request", req))
		return httputil.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	var startDate, endDate time.Time
	startDate, err = utils.ParseStartTime(req.StartDate)
	if err != nil {
		logger.Error("Failed to parse start date", zap.Error(err), zap.String("start_date", req.StartDate))
		return httputil.NewHTTPError(http.StatusBadRequest, "Invalid start date")
	}
	endDate, err = utils.ParseEndTime(req.EndDate)
	if err != nil {
		logger.Error("Failed to parse end date", zap.Error(err), zap.String("end_date", req.EndDate))
		return httputil.NewHTTPError(http.StatusBadRequest, "Invalid end date")
	}

	offset := req.Count * (req.Page - 1)
	var (
		count  int
		topics []Topic
	)

	username, err := contextUsername(c)
	if err != nil {
		return err
	}
	switch req.Type {
	case ContentTypeAnswer:
		answers, err := h.zhihuDBService.FetchAnswerWithDateRange(req.Author, req.Count, offset, req.Order, startDate, endDate)
		if err != nil {
			logger.Error("Failed to fetch answer", zap.Error(err))
			return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to fetch answer")
		}

		topics, err = buildTopicsFromAnswer(answers, username, h.zhihuDBService, h.bookmarkDBService)
		if err != nil {
			logger.Error("Failed to build topics", zap.Error(err))
			return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to build topics")
		}

		count, err = h.zhihuDBService.CountAnswerWithDateRange(req.Author, startDate, endDate)
		if err != nil {
			logger.Error("Failed to count answer", zap.Error(err))
			return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to count answer")
		}
	case ContentTypePin:
		pins, err := h.zhihuDBService.FetchPinWithDateRange(req.Author, req.Count, offset, req.Order, startDate, endDate)
		if err != nil {
			logger.Error("Failed to fetch pin", zap.Error(err))
			return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to fetch pin")
		}

		topics, err = buildTopicsFromPin(pins, username, h.zhihuDBService, h.bookmarkDBService)
		if err != nil {
			logger.Error("Failed to build topics", zap.Error(err))
			return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to build topics")
		}

		count, err = h.zhihuDBService.CountPinWithDateRange(req.Author, startDate, endDate)
		if err != nil {
			logger.Error("Failed to count pin", zap.Error(err))
			return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to count pin")
		}
	case ContentTypeArticle:
	default:
	}

	// calculate page counts (ceil)
	totalPage := (count + req.Count - 1) / req.Count

	return c.JSON(http.StatusOK, httputil.NewResp("success", ArchiveResponse{
		Count:        count,
		Paging:       Paging{Total: totalPage, Current: req.Page},
		ResponseBase: ResponseBase{Topics: topics}}))
}

type archiveResult struct {
	// title is the page title (used for the HTML <title>); markdown is the
	// full-text markdown that every output format is derived from.
	title, markdown, redirectTo string
}

// archive output formats, selected via the `format` query parameter.
const (
	formatHTML     = "html"
	formatMarkdown = "md"
	formatText     = "txt"
)

// parseArchiveFormat normalizes the `format` query parameter. Unknown or empty
// values fall back to HTML to preserve the original behaviour of the endpoint.
func parseArchiveFormat(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "md", "markdown":
		return formatMarkdown
	case "txt", "text", "plain":
		return formatText
	default:
		return formatHTML
	}
}

// buildFormatLinks returns the archive links for all three output formats of the
// same source URL. The source URL is path-escaped so it round-trips back through
// the `:url` route segment (which the handler PathUnescapes).
func buildFormatLinks(sourceURL string) (htmlLink, mdLink, txtLink string) {
	base := fmt.Sprintf("%s/api/v1/archive/%s", config.C.Settings.ServerURL, url.PathEscape(sourceURL))
	return base, base + "?format=md", base + "?format=txt"
}

// formatSwitcherMarkdown renders a one-line cross-format navigation in markdown,
// with the current format shown in bold (not linked) and the others as links.
func formatSwitcherMarkdown(current, htmlLink, mdLink, txtLink string) string {
	htmlPart := fmt.Sprintf("[HTML](%s)", htmlLink)
	mdPart := fmt.Sprintf("[Markdown](%s)", mdLink)
	txtPart := fmt.Sprintf("[纯文本](%s)", txtLink)
	switch current {
	case formatHTML:
		htmlPart = "**HTML**"
	case formatMarkdown:
		mdPart = "**Markdown**"
	case formatText:
		txtPart = "**纯文本**"
	}
	return fmt.Sprintf("格式：%s · %s · %s", htmlPart, mdPart, txtPart)
}

// formatSwitcherText renders the cross-format navigation as plain text (no
// markdown markers), keeping the full URLs so they stay reachable.
func formatSwitcherText(htmlLink, mdLink string) string {
	return fmt.Sprintf("格式：HTML %s · Markdown %s · 纯文本（当前）", htmlLink, mdLink)
}

func (h *Controller) History(c *echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	requestID := c.Response().Header().Get(echo.HeaderXRequestID)

	u, err := echo.PathParam[string](c, "url")
	if err != nil {
		return c.HTML(http.StatusBadRequest, renderErrorPage(err, requestID))
	}
	logger.Info("Raw URL param from request", zap.String("raw_url", u))

	u, err = url.PathUnescape(u)
	if err != nil {
		logger.Error("Failed to unescape url", zap.Error(err))
		return c.HTML(http.StatusBadRequest, renderErrorPage(err, requestID))
	}
	logger.Info("After PathUnescape", zap.String("unescaped_url", u))

	// Fix malformed URLs where double slashes were normalized to single slash by web server/framework
	// e.g., "https:/www.zhihu.com/pin/123" -> "https://www.zhihu.com/pin/123"
	if strings.HasPrefix(u, "https:/") && !strings.HasPrefix(u, "https://") {
		u = strings.Replace(u, "https:/", "https://", 1)
		logger.Info("Fixed normalized https URL", zap.String("fixed_url", u))
	} else if strings.HasPrefix(u, "http:/") && !strings.HasPrefix(u, "http://") {
		u = strings.Replace(u, "http:/", "http://", 1)
		logger.Info("Fixed normalized http URL", zap.String("fixed_url", u))
	}

	logger.Info("Get history url", zap.String("url", u))

	formatValue, err := echo.QueryParamOr[string](c, "format", "")
	if err != nil {
		return c.HTML(http.StatusBadRequest, renderErrorPage(err, requestID))
	}
	format := parseArchiveFormat(formatValue)

	// The `format` parameter selects the output format; any *other* query
	// parameter still triggers the original redirect (it strips tracking
	// params off the source link). The chosen format is preserved across it.
	params := c.Request().URL.Query()
	for k := range params {
		if k == "format" {
			continue
		}
		redirectTo := render.BuildArchiveLink(config.C.Settings.ServerURL, u)
		if format != formatHTML {
			redirectTo += "?format=" + format
		}
		return c.Redirect(http.StatusFound, redirectTo)
	}

	result, err := h.handleRequestArchiveLink(u)
	if err != nil {
		logger.Error("Failed to get webarchive", zap.Error(err))
		status := http.StatusBadRequest
		if errors.Is(err, ErrArchiveNotFound) {
			status = http.StatusNotFound
		}
		return c.HTML(status, renderErrorPage(err, requestID))
	}
	if result.redirectTo != "" {
		return c.Redirect(http.StatusFound, result.redirectTo)
	}

	htmlLink, mdLink, txtLink := buildFormatLinks(u)

	switch format {
	case formatMarkdown:
		switcher := formatSwitcherMarkdown(formatMarkdown, htmlLink, mdLink, txtLink)
		body := switcher + "\n\n---\n\n" + result.markdown
		return c.Blob(http.StatusOK, "text/markdown; charset=utf-8", []byte(body))
	case formatText:
		plain, err := render.Markdown2Text(result.markdown)
		if err != nil {
			logger.Error("Failed to convert markdown to text", zap.Error(err))
			return c.HTML(http.StatusInternalServerError, renderErrorPage(err, requestID))
		}
		body := formatSwitcherText(htmlLink, mdLink) + "\n\n" + plain
		return c.Blob(http.StatusOK, "text/plain; charset=utf-8", []byte(body))
	default:
		switcher := formatSwitcherMarkdown(formatHTML, htmlLink, mdLink, txtLink)
		mdWithNav := switcher + "\n\n---\n\n" + result.markdown
		html, err := h.htmlRender.Render(result.title, mdWithNav)
		if err != nil {
			logger.Error("Failed to render html", zap.Error(err))
			return c.HTML(http.StatusInternalServerError, renderErrorPage(err, requestID))
		}
		return c.HTML(http.StatusOK, html)
	}
}

// ErrArchiveNotFound signals that the link was well-formed but the resource is
// not archived. History maps it to 404 (other handler errors map to 400).
var ErrArchiveNotFound = errors.New("archive: resource not found")

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
	case tk.IsWeiboArchiveLink(link):
		return h.HandleTombkeeperWeibo(link)
	case tkblog.IsBlogArchiveLink(link):
		return h.HandleTkblog(link)
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
