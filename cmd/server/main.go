package main

import (
	"net"

	"github.com/eli-yip/rss-zero/cmd/server/controller"
	myMiddleware "github.com/eli-yip/rss-zero/cmd/server/middleware"
	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/cron"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/log"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func main() {
	var err error
	logger := log.NewLogger()

	config.InitConfigFromEnv()
	logger.Info("config initialized")

	redisService, err := redis.NewRedisService(config.C.RedisAddr, "", 0)
	if err != nil {
		logger.Fatal("redis service init failed", zap.Error(err))
	}
	logger.Info("redis service initialized")

	db, err := db.NewDB(config.C.DBHost, config.C.DBPort, config.C.DBUser, config.C.DBPassword, config.C.DBName)
	if err != nil {
		logger.Fatal("db init failed", zap.Error(err))
	}

	bark := notify.NewBarkNotifier(config.C.BarkURL)
	setupCron(logger, redisService, db, bark)

	e := setupEcho(redisService, db, bark, logger)

	err = e.Start(":8080")
	if err != nil {
		panic(err)
	}
}

func setupEcho(redisService *redis.RedisService,
	db *gorm.DB,
	notifier notify.Notifier,
	logger *zap.Logger) *echo.Echo {
	e := echo.New()
	e.IPExtractor = echo.ExtractIPFromXFFHeader(
		echo.TrustIPRange(func(ip string) *net.IPNet {
			_, ipNet, _ := net.ParseCIDR(ip)
			return ipNet
		}("172.0.0.0/8")),
	)
	e.Use(middleware.RequestID(), middleware.Recover(),
		myMiddleware.LogRequest(logger), myMiddleware.InjectLogger(logger))

	zsxqHandler := controller.NewZsxqHandler(redisService, db, notifier, logger)

	rssGroup := e.Group("/rss")
	rssZsxq := rssGroup.GET("/zsxq/:id", zsxqHandler.Get)
	rssZsxq.Name = "RSS route for zsxq"

	exportGroup := e.Group("/export")
	exportZsxq := exportGroup.POST("/zsxq", zsxqHandler.ExportZsxq)
	exportZsxq.Name = "Export route for zsxq"

	refmtGroup := e.Group("/refmt")
	refmtZsxq := refmtGroup.POST("/zsxq", zsxqHandler.Refmt)
	refmtZsxq.Name = "Re-format route for zsxq"

	cookieGroup := e.Group("/cookie")
	cookieZsxq := cookieGroup.POST("/zsxq", zsxqHandler.UpdateZsxqCookies)
	cookieZsxq.Name = "Update cookies route for zsxq"

	return e
}

func setupCron(logger *zap.Logger, redis *redis.RedisService, db *gorm.DB, notifier notify.Notifier) {
	s, err := cron.NewCronService(logger)
	if err != nil {
		logger.Fatal("cron service init failed", zap.Error(err))
	}

	err = s.AddJob(cron.CrawlZsxq(redis, db, notifier))
	if err != nil {
		logger.Fatal("add job failed", zap.Error(err))
	}
}
