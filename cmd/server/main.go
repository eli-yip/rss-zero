package main

import (
	"time"

	middleware "github.com/eli-yip/rss-zero/cmd/server/middleware"
	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/cron"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/log"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/middleware/recover"
	"github.com/kataras/iris/v12/middleware/requestid"
	"github.com/rs/cors"
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

	app := setupApp(redisService, db, bark, logger)

	routes := app.GetRoutes()
	for _, r := range routes {
		logger.Info("route registered", zap.String("method", r.Method), zap.String("path", r.Path))
	}

	err = app.Listen(":8080", iris.WithLowercaseRouting)
	if err != nil {
		panic(err)
	}
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

func setupApp(redisService *redis.RedisService,
	db *gorm.DB,
	notifier notify.Notifier,
	logger *zap.Logger) (app *iris.Application) {
	app = iris.New()

	corsOpts := cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedHeaders:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS"},
		ExposedHeaders:   []string{"X-Header"},
		AllowCredentials: true,
		MaxAge:           int((24 * time.Hour).Seconds()),
		Debug:            false,
	}

	c := cors.New(corsOpts)
	app.WrapRouter(c.ServeHTTP)

	app.Use(requestid.New(), recover.New(),
		middleware.LogRequest(logger), middleware.LoggerMiddleware(logger))

	rss := app.Party("/rss")
	zsxq := rss.Party("/zsxq")

	zsxqHandler := NewZsxqHandler(redisService, db, logger)
	zsxq.Get("/{id:string}", zsxqHandler.Get)

	cookies := app.Party("/cookies")
	zsxqCookiesHandler := NewCookiesHandler(redisService)
	cookies.Post("/zsxq", zsxqCookiesHandler.UpdateZsxqCookies)

	refmt := app.Party("/refmt")
	refmtHandler := NewRefmtHandler(db, notifier)
	refmt.Post("/zsxq", refmtHandler.Post)

	export := app.Party("/export")
	exportHandler := NewExportHandler(db, logger, notifier)
	export.Post("/zsxq", exportHandler.ExportZsxq)

	return app
}
