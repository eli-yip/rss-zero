package crawl

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"go.uber.org/zap"
)

type listPageParser[T any] func([]byte, int, *zap.Logger) (apiModels.Paging, []T, []json.RawMessage, error)
type listItemParser[T any] func(T, json.RawMessage, *zap.Logger) error
type listItemSkipper[T any] func(T, *zap.Logger) bool

type listCrawlOptions[T any] struct {
	contentType        string
	startMessage       string
	reachTargetMessage string
	reachEndMessage    string
	foundNewMessage    string
	foundNewCountField string
	foundNewError      string
	parseList          listPageParser[T]
	parseItem          listItemParser[T]
	skipItem           listItemSkipper[T]
	createdAt          func(T) int64
	itemLogFields      func(T) []zap.Field
	parseListLogFields func(string) []zap.Field
	generateURL        func(string, int) string
}

func crawlListPages[T any](user string, rs request.Requester, targetTime time.Time, offset int, oneTime bool, logger *zap.Logger, opts listCrawlOptions[T]) error {
	logger.Info(opts.startMessage, zap.String("user_url_token", user))

	next := opts.generateURL(user, offset)

	index := 0
	lastTotalCount := 0
	for {
		bytes, err := rs.LimitRaw(context.Background(), next, logger)
		if err != nil {
			logger.Error("Failed to request zhihu api", zap.Error(err), zap.String("url", next))
			return fmt.Errorf("failed to request zhihu api: %w", err)
		}
		logger.Info("Request zhihu api successfully", zap.String("url", next))

		paging, excerpts, rawItems, err := opts.parseList(bytes, index, logger)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to parse %s list", opts.contentType), zap.Error(err), zap.Int("index", index), zap.String("url", next))
			return fmt.Errorf("failed to parse %s list: %w", opts.contentType, err)
		}

		parseListFields := []zap.Field{zap.Int("index", index)}
		if opts.parseListLogFields != nil {
			parseListFields = append(parseListFields, opts.parseListLogFields(next)...)
		}
		logger.Info(fmt.Sprintf("Parse %s list successfully", opts.contentType), parseListFields...)

		if index != 0 && paging.Totals != lastTotalCount {
			logger.Error(opts.foundNewMessage, zap.Int(opts.foundNewCountField, paging.Totals-lastTotalCount))
			return fmt.Errorf("%s", opts.foundNewError)
		}
		lastTotalCount = paging.Totals

		next = paging.Next

		for i, item := range slices.Backward(excerpts) {
			itemLogger := logger.With(opts.itemLogFields(item)...)

			if opts.skipItem != nil && opts.skipItem(item, itemLogger) {
				continue
			}

			if err = opts.parseItem(item, rawItems[i], itemLogger); err != nil {
				itemLogger.Error(fmt.Sprintf("Failed to parse %s", opts.contentType), zap.Error(err))
				return fmt.Errorf("failed to parse %s: %w", opts.contentType, err)
			}
			itemLogger.Info(fmt.Sprintf("Parse %s successfully", opts.contentType))
		}

		if len(excerpts) > 0 && !time.Unix(opts.createdAt(excerpts[len(excerpts)-1]), 0).After(targetTime) {
			logger.Info(opts.reachTargetMessage)
			return nil
		}

		if paging.IsEnd {
			logger.Info(opts.reachEndMessage)
			break
		}

		index++

		if oneTime {
			logger.Info("One time mode, break")
			break
		}
	}

	return nil
}
