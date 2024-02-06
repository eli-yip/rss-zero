package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

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

	redisService, err := redis.NewRedisService(config.C.RedisAddr, "", 0)
	if err != nil {
		logger.Fatal("redis service init failed", zap.Error(err))
	}
	logger.Info("redis service initialized")

	db, err := db.NewDB(config.C.DBHost, config.C.DBPort, config.C.DBUser, config.C.DBPassword, config.C.DBName)
	if err != nil {
		logger.Fatal("db init failed", zap.Error(err))
	}
	logger.Info("db initialized")

	bark := notify.NewBarkNotifier(config.C.BarkURL)
	setupCron(logger, redisService, db, bark)
	logger.Info("cron service initialized")

	e := setupEcho(redisService, db, bark, logger)
	logger.Info("echo server initialized")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
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
}

func setupEcho(redisService *redis.RedisService,
	db *gorm.DB,
	notifier notify.Notifier,
	logger *zap.Logger) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.IPExtractor = echo.ExtractIPFromXFFHeader(
		echo.TrustIPRange(func(ip string) *net.IPNet {
			_, ipNet, _ := net.ParseCIDR(ip)
			return ipNet
		}("172.0.0.0/8")),
	)
	e.Use(middleware.RequestID(), middleware.Recover(),
		myMiddleware.LogRequest(logger), myMiddleware.InjectLogger(logger))

	zsxqHandler := controller.NewZsxqHandler(redisService, db, notifier, logger)
	zhihuDB := zhihuDB.NewDBService(db)
	zhihuHandler := controller.NewZhihuHandler(redisService, zhihuDB, notifier, logger)

	rssGroup := e.Group("/rss")
	rssGroup.Use(myMiddleware.SetRSSContentType(), myMiddleware.ExtractFeedID())
	rssZsxq := rssGroup.GET("/zsxq/:id", zsxqHandler.RSS)
	rssZsxq.Name = "RSS route for zsxq"
	rssZhihu := rssGroup.Group("/zhihu")
	rssZhihuAnswer := rssZhihu.GET("/answer/:id", zhihuHandler.AnswerRSS)
	rssZhihuAnswer.Name = "RSS route for zhihu answer"
	rssZhihuArticle := rssZhihu.GET("/article/:id", zhihuHandler.ArticleRSS)
	rssZhihuArticle.Name = "RSS route for zhihu article"
	rssZhihuPin := rssZhihu.GET("/pin/:id", zhihuHandler.PinRSS)
	rssZhihuPin.Name = "RSS route for zhihu pin"

	exportGroup := e.Group("/export")
	exportZsxq := exportGroup.POST("/zsxq", zsxqHandler.Export)
	exportZsxq.Name = "Export route for zsxq"
	exportZhihu := exportGroup.POST("/zhihu", zhihuHandler.Export)
	exportZhihu.Name = "Export route for zhihu"

	refmtGroup := e.Group("/refmt")
	refmtZsxq := refmtGroup.POST("/zsxq", zsxqHandler.Refmt)
	refmtZsxq.Name = "Re-format route for zsxq"
	refmtZhihu := refmtGroup.POST("/zhihu", zhihuHandler.Refmt)
	refmtZhihu.Name = "Re-format route for zhihu"

	cookieGroup := e.Group("/cookie")
	cookieZsxq := cookieGroup.POST("/zsxq", zsxqHandler.UpdateZsxqCookies)
	cookieZsxq.Name = "Update cookies route for zsxq"

	apiGroup := e.Group("/api/v1")
	feedApi := apiGroup.Group("/feed")
	zhihuFeedApi := feedApi.GET("/zhihu/:id", zhihuHandler.Feed)
	zhihuFeedApi.Name = "Feed route for zhihu"

	return e
}

func setupCron(logger *zap.Logger, redis *redis.RedisService, db *gorm.DB, notifier notify.Notifier) {
	s, err := cron.NewCronService(logger)
	if err != nil {
		logger.Fatal("cron service init failed", zap.Error(err))
	}

	if err = s.AddJob(cron.CrawlZsxq(redis, db, notifier)); err != nil {
		logger.Fatal("add zsxq job failed", zap.Error(err))
	}

	if err = s.AddJob(cron.CrawlZhihu(redis, db, notifier)); err != nil {
		logger.Fatal("add zhihu job failed", zap.Error(err))
	}
}
