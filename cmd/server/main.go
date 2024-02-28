package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/brpaz/echozap"
	"github.com/eli-yip/rss-zero/cmd/server/controller"
	myMiddleware "github.com/eli-yip/rss-zero/cmd/server/middleware"
	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/cron"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/log"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func main() {
	var err error

	redisService, db, bark, logger, err := initService()
	if err != nil {
		logger.Fatal("fail to init service", zap.Error(err))
	}
	logger.Info("service initialized")

	if err = setupCron(logger, redisService, db, bark); err != nil {
		logger.Fatal("fail to setup cron", zap.Error(err))
	}
	logger.Info("cron service initialized")

	e := setupEcho(redisService, db, bark, logger)
	logger.Info("echo server initialized")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		logger.Info("start server", zap.String("address", ":8080"))
		if err := e.Start(":8080"); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()

	<-ctx.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}

	logger.Info("server shutdown")
}

// initService initializes services
//
// r: redis service
//
// d: postgres db
//
// n: notifier
//
// l: logger
func initService() (r redis.Redis,
	d *gorm.DB,
	n notify.Notifier,
	l *zap.Logger,
	err error) {
	l = log.NewZapLogger()

	config.InitFromEnv()
	l.Info("config initialized", zap.Any("config", config.C))

	r, err = redis.NewRedisService(config.C.Redis)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("fail to init redis service: %w", err)
	}
	l.Info("redis service initialized")

	d, err = db.NewPostgresDB(config.C.DB)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("fail to init db: %w", err)
	}
	l.Info("db initialized")

	bark := notify.NewBarkNotifier(config.C.BarkURL)
	l.Info("bark notifier initialized")

	return r, d, bark, l, nil
}

func setupEcho(redisService redis.Redis,
	db *gorm.DB,
	notifier notify.Notifier,
	logger *zap.Logger) (e *echo.Echo) {
	e = echo.New()
	e.HideBanner = true
	e.IPExtractor = echo.ExtractIPFromXFFHeader(
		echo.TrustIPRange(func(ip string) *net.IPNet {
			_, ipNet, _ := net.ParseCIDR(ip)
			return ipNet
		}("172.0.0.0/8")), // trust docker network
	)
	e.Use(
		echozap.ZapLogger(logger),         // use zap for echo logger
		middleware.RequestID(),            // add request id
		middleware.Recover(),              // recover from panic
		myMiddleware.LogRequest(logger),   // log request
		myMiddleware.InjectLogger(logger), // inject logger to context
	)

	zsxqHandler := controller.NewZsxqHandler(redisService, db, notifier, logger)

	zhihuDBService := zhihuDB.NewDBService(db)
	zhihuHandler := controller.NewZhihuHandler(redisService, zhihuDBService, notifier, logger)
	xiaobotDBService := xiaobotDB.NewDBService(db)
	xiaobotHandler := controller.NewXiaobotController(redisService, xiaobotDBService, notifier, logger)

	// /rss
	rssGroup := e.Group("/rss")
	rssGroup.Use(
		myMiddleware.SetRSSContentType(), // set content type to application/atom+xml
		myMiddleware.ExtractFeedID(),     // extract feed id from url and set it to context
	)

	// /rss/zsxq/:feed
	rssZsxq := rssGroup.GET("/zsxq/:feed", zsxqHandler.RSS)
	rssZsxq.Name = "RSS route for zsxq group"

	// /rss/zhihu/:feed
	rssZhihu := rssGroup.Group("/zhihu")
	// /rss/zhihu/answer/:feed
	rssZhihuAnswer := rssZhihu.GET("/answer/:feed", zhihuHandler.AnswerRSS)
	rssZhihuAnswer.Name = "RSS route for zhihu answer"
	// /rss/zhihu/article/:feed
	rssZhihuArticle := rssZhihu.GET("/article/:feed", zhihuHandler.ArticleRSS)
	rssZhihuArticle.Name = "RSS route for zhihu article"
	// /rss/zhihu/pin/:feed
	rssZhihuPin := rssZhihu.GET("/pin/:feed", zhihuHandler.PinRSS)
	rssZhihuPin.Name = "RSS route for zhihu pin"

	rssXiaobot := rssGroup.GET("/xiaobot/:feed", xiaobotHandler.RSS)
	rssXiaobot.Name = "RSS route for xiaobot"

	// /api/v1
	apiGroup := e.Group("/api/v1")

	// /api/v1/feed
	feedApi := apiGroup.Group("/feed")
	// /api/v1/feed/zhihu/:id
	zhihuFeedApi := feedApi.GET("/zhihu/:id", zhihuHandler.Feed)
	zhihuFeedApi.Name = "Feed route for zhihu"

	// /api/v1/cookie
	cookieApi := apiGroup.Group("/cookie")
	// /api/v1/cookie/zsxq
	zsxqCookieApi := cookieApi.POST("/zsxq", zsxqHandler.UpdateZsxqCookie)
	zsxqCookieApi.Name = "Cookie updating route for zsxq"
	// /api/v1/cookie/xiaobot
	xiaobotCookieApi := cookieApi.POST("/xiaobot", xiaobotHandler.UpdateToken)
	xiaobotCookieApi.Name = "Token updating route for xiaobot"

	// /api/v1/refmt
	refmtApi := apiGroup.Group("/refmt")
	// /api/v1/refmt/zsxq
	refmtZsxqApi := refmtApi.POST("/zsxq", zsxqHandler.Reformat)
	refmtZsxqApi.Name = "Reformat route for zsxq"
	// /api/v1/refmt/zhihu
	refmtZhihuApi := refmtApi.POST("/zhihu", zhihuHandler.Reformat)
	refmtZhihuApi.Name = "Reformat route for zhihu"
	// /api/v1/refmt/xiaobot
	refmtXiaobotApi := refmtApi.POST("/xiaobot", xiaobotHandler.Reformat)
	refmtXiaobotApi.Name = "Reformat route for xiaobot"

	// /api/v1/export
	exportApi := apiGroup.Group("/export")
	// /api/v1/export/zsxq
	exportZsxqApi := exportApi.POST("/zsxq", zsxqHandler.Export)
	exportZsxqApi.Name = "Export route for zsxq"
	// /api/v1/export/zhihu
	exportZhihuApi := exportApi.POST("/zhihu", zhihuHandler.Export)
	exportZhihuApi.Name = "Export route for zhihu"
	// /api/v1/export/xiaobot
	exportXiaobotApi := exportApi.POST("/xiaobot", xiaobotHandler.Export)
	exportXiaobotApi.Name = "Export route for xiaobot"

	healthEndpoint := apiGroup.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, struct {
			Status string `json:"status"`
		}{Status: "ok"})
	})
	healthEndpoint.Name = "Health check route"

	// iterate all routes and log them
	for _, r := range e.Routes() {
		logger.Info("route",
			zap.String("name", r.Name),
			zap.String("path", r.Path))
	}

	return e
}

// setupCron sets up cron jobs
func setupCron(logger *zap.Logger,
	redisService redis.Redis,
	db *gorm.DB,
	notifier notify.Notifier,
) (err error) {
	type cronFunc func(redis.Redis, *gorm.DB, notify.Notifier) func()
	cronFuncs := []cronFunc{
		cron.CrawlZsxq,
		cron.CrawlZhihu,
		cron.CrawlXiaobot,
	}

	s, err := cron.NewCronService(logger)
	if err != nil {
		return fmt.Errorf("cron service init failed: %w", err)
	}

	for _, f := range cronFuncs {
		if err = s.AddJob(f(redisService, db, notifier)); err != nil {
			return fmt.Errorf("fail to add job: %w", err)
		}
	}

	return nil
}
