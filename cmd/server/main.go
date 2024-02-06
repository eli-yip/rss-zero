package main

import (
	"context"
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
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func main() {
	var err error
	logger := log.NewLogger()

	config.InitFromEnv()
	logger.Info("config initialized", zap.Any("config", config.C))

	redisService, err := redis.NewRedisService(config.C.Redis)
	if err != nil {
		logger.Fatal("redis service init failed", zap.Error(err))
	}
	logger.Info("redis service initialized")

	db, err := db.NewPostgresDB(config.C.DB)
	if err != nil {
		logger.Fatal("db init failed", zap.Error(err))
	}
	logger.Info("db initialized")

	bark := notify.NewBarkNotifier(config.C.BarkURL)
	logger.Info("bark notifier initialized")

	setupCron(logger, redisService, db, bark)
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

func setupEcho(redisService *redis.RedisService,
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

	// /api/v1/refmt
	refmtApi := apiGroup.Group("/refmt")
	// /api/v1/refmt/zsxq
	refmtZsxqApi := refmtApi.POST("/zsxq", zsxqHandler.Reformat)
	refmtZsxqApi.Name = "Reformat route for zsxq"
	// /api/v1/refmt/zhihu
	refmtZhihuApi := refmtApi.POST("/zhihu", zhihuHandler.Reformat)
	refmtZhihuApi.Name = "Reformat route for zhihu"

	// /api/v1/export
	exportApi := apiGroup.Group("/export")
	// /api/v1/export/zsxq
	exportZsxqApi := exportApi.POST("/zsxq", zsxqHandler.Export)
	exportZsxqApi.Name = "Export route for zsxq"
	// /api/v1/export/zhihu
	exportZhihuApi := exportApi.POST("/zhihu", zhihuHandler.Export)
	exportZhihuApi.Name = "Export route for zhihu"

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
func setupCron(logger *zap.Logger, redisService *redis.RedisService, db *gorm.DB, notifier notify.Notifier) {
	type cronFunc func(*redis.RedisService, *gorm.DB, notify.Notifier) func()
	cronFuncs := []cronFunc{
		cron.CrawlZsxq,
		cron.CrawlZhihu,
	}

	s, err := cron.NewCronService(logger)
	if err != nil {
		logger.Fatal("cron service init failed", zap.Error(err))
	}

	for _, f := range cronFuncs {
		if err = s.AddJob(f(redisService, db, notifier)); err != nil {
			logger.Fatal("add job failed", zap.Error(err))
		}
	}
}
