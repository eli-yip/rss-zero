package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	jobController "github.com/eli-yip/rss-zero/internal/controller/job"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/migrate"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/version"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/cron"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

var configPath string

func parseFlags() {
	flag.StringVar(&configPath, "config", "", "path to the config file")
	flag.Parse()
}

func main() {
	var err error

	parseFlags()
	if strings.HasSuffix(configPath, ".toml") {
		if err = config.InitFromToml(configPath); err != nil {
			panic("failed to init config from file: " + err.Error())
		}
	} else {
		panic("invalid config file extension: " + configPath + ", only `.toml` is supported")
	}

	logger := log.NewZapLogger()
	logger.Info("Init config from toml successfully", zap.Any("config", config.C))

	// Add global recover
	func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Fatal("Panic", zap.Any("panic", r))
			}
		}()
	}()

	redisService, cookieService, db, bark, err := initService(logger)
	if err != nil {
		logger.Fatal("Failed to init service", zap.Error(err))
	}
	logger.Info("Init services successfully")

	n, err := zhihuDB.SetEmptySubID(db)
	if err != nil {
		logger.Fatal("Failed to set empty sub id", zap.Error(err))
	}
	if n != 0 {
		logger.Info("Set empty sub id successfully", zap.Int("count", n))
	} else {
		logger.Info("No empty sub id found")
	}

	var definitionToFunc jobController.DefinitionToFunc
	var cronService *cron.CronService
	if cronService, definitionToFunc, err = setupCronCrawlJob(logger, redisService, cookieService, db, bark); err != nil {
		logger.Fatal("Failed to setup cron jobs", zap.Error(err))
	}
	logger.Info("Init cron service and jobs successfully")

	e := setupEcho(redisService, cookieService, db, bark, definitionToFunc, cronService, logger)
	logger.Info("Init echo server successfully")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Start echo server
	go func() {
		logger.Info("Start server now!", zap.String("address", ":8080"), zap.String("version", version.Version))
		if err := e.Start(":8080"); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Shutdown server", zap.Error(err))
		}
	}()

	<-ctx.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err = e.Shutdown(ctx); err != nil {
		logger.Fatal("Failed to shutdown server", zap.Error(err))
	}

	logger.Info("Shutdown server successfully")
}

func initService(logger *zap.Logger) (redisService redis.Redis,
	cookieService cookie.CookieIface,
	dbService *gorm.DB,
	notifier notify.Notifier,
	err error) {
	if redisService, err = redis.NewRedisService(config.C.Redis); err != nil {
		logger.Error("Failed to init redis service", zap.Error(err))
		return nil, nil, nil, nil, fmt.Errorf("failed to init redis service: %w", err)
	}
	logger.Info("redis service initialized")

	if dbService, err = db.NewPostgresDB(config.C.Database); err != nil {
		logger.Error("Failed to init postgres database service", zap.Error(err))
		return nil, nil, nil, nil, fmt.Errorf("failed to init db: %w", err)
	}
	logger.Info("db initialized")

	if err = migrate.MigrateDB(dbService); err != nil {
		logger.Error("Failed to migrate database", zap.Error(err))
		return nil, nil, nil, nil, fmt.Errorf("failed to migrate db: %w", err)
	}

	cookieService = cookie.NewCookieService(dbService)

	notifier = notify.NewBarkNotifier(config.C.Bark.URL)
	logger.Info("bark notifier initialized")

	return redisService, cookieService, dbService, notifier, nil
}
