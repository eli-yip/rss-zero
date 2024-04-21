package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/brpaz/echozap"
	endoflifeController "github.com/eli-yip/rss-zero/cmd/server/controller/endoflife"
	rsshubController "github.com/eli-yip/rss-zero/cmd/server/controller/rsshub"
	xiaobotController "github.com/eli-yip/rss-zero/cmd/server/controller/xiaobot"
	zhihuController "github.com/eli-yip/rss-zero/cmd/server/controller/zhihu"
	zsxqController "github.com/eli-yip/rss-zero/cmd/server/controller/zsxq"
	myMiddleware "github.com/eli-yip/rss-zero/cmd/server/middleware"
	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/cron"
	xiaobotCron "github.com/eli-yip/rss-zero/internal/cron/xiaobot"
	zhihuCron "github.com/eli-yip/rss-zero/internal/cron/zhihu"
	zsxqCron "github.com/eli-yip/rss-zero/internal/cron/zsxq"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/version"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
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

	// Start echo server
	go func() {
		logger.Info("start server", zap.String("address", ":8080"), zap.String("version", version.Version))
		if err := e.Start(":8080"); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal("shutting down the server")
		}
	}()

	<-ctx.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err = e.Shutdown(ctx); err != nil {
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
// logger: logger
func initService() (redisService redis.Redis,
	dbService *gorm.DB,
	notifier notify.Notifier,
	logger *zap.Logger,
	err error) {
	config.InitFromEnv()

	logger = log.NewZapLogger()
	logger.Info("Init zap logger", zap.Bool("Debug Mode", config.C.Debug))
	logger.Info("config initialized", zap.Any("config", config.C))

	if redisService, err = redis.NewRedisService(config.C.Redis); err != nil {
		logger.Error("Fail to init redis service", zap.Error(err))
		return nil, nil, nil, nil, fmt.Errorf("fail to init redis service: %w", err)
	}
	logger.Info("redis service initialized")

	if dbService, err = db.NewPostgresDB(config.C.DB); err != nil {
		logger.Error("Fail to init postgres database service", zap.Error(err))
		return nil, nil, nil, nil, fmt.Errorf("fail to init db: %w", err)
	}
	logger.Info("db initialized")

	notifier = notify.NewBarkNotifier(config.C.BarkURL)
	logger.Info("bark notifier initialized")

	return redisService, dbService, notifier, logger, nil
}

func setupEcho(redisService redis.Redis,
	db *gorm.DB,
	notifier notify.Notifier,
	logger *zap.Logger) (e *echo.Echo) {
	e = echo.New()
	e.HideBanner = true
	e.HidePort = true
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

	zsxqHandler := zsxqController.NewZsxqHandler(redisService, db, notifier, logger)

	zhihuDBService := zhihuDB.NewDBService(db)
	zhihuHandler := zhihuController.NewZhihuHandler(redisService, zhihuDBService, notifier, logger)
	xiaobotDBService := xiaobotDB.NewDBService(db)
	xiaobotHandler := xiaobotController.NewXiaobotController(redisService, xiaobotDBService, notifier, logger)
	endOfLifeHandler := endoflifeController.NewController(redisService, logger)

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

	rssEndOfLife := rssGroup.GET("/endoflife/:feed", endOfLifeHandler.RSS)
	rssEndOfLife.Name = "RSS route for endoflife.date"

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
	// comment this because not needed now
	// // /api/v1/cookie/zhihu
	// zhihuCookieApi := cookieApi.POST("/zhihu", zhihuHandler.UpdateZhihuCookie)
	// zhihuCookieApi.Name = "Cookie updating route for zhihu"

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

	// /api/v1/rsshub
	rssHubApi := apiGroup.Group("/rsshub")
	feedGeneratorApi := rssHubApi.POST("/feed", rsshubController.GenerateRSSHubFeed)
	feedGeneratorApi.Name = "RSSHub feed generator route"

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
	type cronJob struct {
		name string
		fn   cronFunc
	}
	cronjobs := []cronJob{
		{"zsxq crawl", zsxqCron.Cron},
		{"zhihu crawl", zhihuCron.CrawlZhihu},
		{"xiaobot crawl", xiaobotCron.CrawlXiaobot},
	}

	cronService, err := cron.NewCronService(logger)
	if err != nil {
		return fmt.Errorf("cron service init failed: %w", err)
	}

	for _, job := range cronjobs {
		if err = cronService.AddJob(job.name, job.fn(redisService, db, notifier)); err != nil {
			return fmt.Errorf("fail to add job: %w", err)
		}
	}

	return nil
}
