package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	serverCommon "github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	zhihuCrawl "github.com/eli-yip/rss-zero/pkg/routers/zhihu/crawl"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

// AnswerRSS / ArticleRSS / PinRSS serve a zhihu author's feed of that content type
// through the unified pipeline. Each ensures the subscription exists (adding the
// author on first request, 400 if the author is unknown to zhihu) before Serve.
func (h *Controller) AnswerRSS(c echo.Context) error  { return h.serveZhihu(c, common.ZhihuAnswer) }
func (h *Controller) ArticleRSS(c echo.Context) error { return h.serveZhihu(c, common.ZhihuArticle) }
func (h *Controller) PinRSS(c echo.Context) error     { return h.serveZhihu(c, common.ZhihuPin) }

func (h *Controller) serveZhihu(c echo.Context, contentType common.ZhihuContentType) error {
	logger := serverCommon.ExtractLogger(c)

	authorID := c.Get("feed_id").(string)
	logger.Info("Retrieve rss request", zap.String("author_id", authorID))

	if err := h.checkSub(contentType, authorID, logger); err != nil {
		if errors.Is(err, errAuthorNotExistInZhihu) {
			logger.Error("Failed to find author in zhihu website", zap.String("author_id", authorID))
			return httputil.NewHTTPError(http.StatusBadRequest, "Author does not exist in zhihu website")
		}
		logger.Error("Failed to check sub", zap.Error(err))
		return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to check sub")
	}

	return rss.Serve(c, rss.ServeOptions{
		Redis:        h.redis,
		Logger:       logger,
		Key:          contentType.RedisKey(authorID),
		TTL:          redis.RSSDefaultTTL,
		DefaultLimit: 20,
		Fetch: func() (rss.FeedMeta, []rss.Item, error) {
			return rss.FetchZhihu(contentType, authorID, h.db, logger)
		},
	})
}

// checkSub checks if the sub exists in db, if not, add it to db.
func (h *Controller) checkSub(t common.ZhihuContentType, authorID string, logger *zap.Logger) (err error) {
	// Use CheckSubIncludeDeleted instead of CheckSubByID to check if the sub exists,
	// as we will return history rss content even if the sub is deleted.
	exist, err := h.db.CheckSubIncludeDeleted(authorID, t)
	if err != nil {
		return fmt.Errorf("failed to check sub: %w", err)
	}
	logger.Info("Check zhihu subscription successfully", zap.Bool("exist", exist))

	if exist {
		return nil
	}

	logger.Info("Start to add zhihu subscription")
	if _, err = h.parseAuthorName(authorID, logger); err != nil {
		return fmt.Errorf("failed to parse author name: %w", err)
	}

	if err = h.db.AddSub(authorID, t); err != nil {
		return fmt.Errorf("failed to add sub: %w", err)
	}

	return nil
}

var errAuthorNotExistInZhihu = errors.New("author does not exist in zhihu")

// parseAuthorName parses author name from authorID, and returns the author name.
// It will save the author name to db if it's not found in db.
func (h *Controller) parseAuthorName(authorID string, logger *zap.Logger) (authorName string, err error) {
	zhihuCookies, err := cookie.GetZhihuCookies(h.cookie, logger)
	if err != nil {
		return "", fmt.Errorf("failed to get zhihu cookies: %w", err)
	}
	logger.Info("Get zhihu cookies successfully", zap.Any("cookies", zhihuCookies))

	requestService, err := request.NewRequestService(logger, h.db, notify.NewBarkNotifier(config.C.Bark.URL), zhihuCookies, request.WithLimiter(request.NewLimiter()))
	if err != nil {
		return "", fmt.Errorf("failed to create request service: %w", err)
	}

	bytes, err := requestService.LimitRaw(context.Background(), zhihuCrawl.GenerateAnswerApiURL(authorID, 0), logger)
	if err != nil {
		if errors.Is(err, request.ErrUnreachable) {
			logger.Info("Author does not exist in zhihu website", zap.String("author_id", authorID))
			return "", errAuthorNotExistInZhihu
		}
		return "", fmt.Errorf("failed to get author name: %w", err)
	}

	var parser parse.AuthorParser
	parser, err = parse.NewParseService(parse.WithDB(h.db))
	if err != nil {
		return "", fmt.Errorf("failed to create parse service: %w", err)
	}
	logger.Info("Create parse service successfully")

	authorName, err = parser.ParseAuthorName(bytes, logger)
	if err != nil {
		return "", fmt.Errorf("failed to parse author name: %w", err)
	}
	logger.Info("Get author name from zhihu successfully", zap.String("author_name", authorName))

	return authorName, nil
}
