package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"gorm.io/gorm"

	archiveController "github.com/eli-yip/rss-zero/cmd/server/controller/archive"
	endoflifeController "github.com/eli-yip/rss-zero/cmd/server/controller/endoflife"
	jobController "github.com/eli-yip/rss-zero/cmd/server/controller/job"
	rsshubController "github.com/eli-yip/rss-zero/cmd/server/controller/rsshub"
	xiaobotController "github.com/eli-yip/rss-zero/cmd/server/controller/xiaobot"
	zhihuController "github.com/eli-yip/rss-zero/cmd/server/controller/zhihu"
	zsxqController "github.com/eli-yip/rss-zero/cmd/server/controller/zsxq"
	myMiddleware "github.com/eli-yip/rss-zero/cmd/server/middleware"
	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/version"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	xiaobotCron "github.com/eli-yip/rss-zero/pkg/cron/xiaobot"
	zhihuCron "github.com/eli-yip/rss-zero/pkg/cron/zhihu"
	zsxqCron "github.com/eli-yip/rss-zero/pkg/cron/zsxq"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
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
	logger.Info("config initialized", zap.Any("config", config.C))

	redisService, db, bark, err := initService(logger)
	if err != nil {
		logger.Fatal("fail to init service", zap.Error(err))
	}
	logger.Info("service initialized")

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
	if definitionToFunc, err = setupCronCrawlJob(logger, redisService, db, bark); err != nil {
		logger.Fatal("fail to setup cron", zap.Error(err))
	}
	logger.Info("cron service initialized")

	e := setupEcho(redisService, db, bark, definitionToFunc, logger)
	logger.Info("echo server initialized")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Start echo server
	go func() {
		logger.Info("start server", zap.String("address", ":8080"), zap.String("version", version.Version))
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

// initService initializes services
//
// r: redis service
//
// d: postgres db
//
// n: notifier
func initService(logger *zap.Logger) (redisService redis.Redis,
	dbService *gorm.DB,
	notifier notify.Notifier,
	err error) {
	if redisService, err = redis.NewRedisService(config.C.Redis); err != nil {
		logger.Error("Fail to init redis service", zap.Error(err))
		return nil, nil, nil, fmt.Errorf("fail to init redis service: %w", err)
	}
	logger.Info("redis service initialized")

	if dbService, err = db.NewPostgresDB(config.C.Database); err != nil {
		logger.Error("Fail to init postgres database service", zap.Error(err))
		return nil, nil, nil, fmt.Errorf("fail to init db: %w", err)
	}
	logger.Info("db initialized")

	notifier = notify.NewBarkNotifier(config.C.Bark.URL)
	logger.Info("bark notifier initialized")

	return redisService, dbService, notifier, nil
}

func setupEcho(redisService redis.Redis,
	db *gorm.DB,
	notifier notify.Notifier,
	definitionToFunc jobController.DefinitionToFunc,
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
	cronDBService := cronDB.NewDBService(db)
	jobHandler := jobController.NewController(cronDBService, definitionToFunc, logger)

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
	// /api/v1/cookie/zhihu
	zhihuCookieApi := cookieApi.POST("/zhihu", zhihuHandler.UpdateCookie)
	zhihuCookieApi.Name = "Cookie updating route for zhihu"
	// /api/v1/cookie/zhihu/check
	zhihuCheckCookieApi := cookieApi.GET("/zhihu", zhihuHandler.CheckCookie)
	zhihuCheckCookieApi.Name = "Cookie checking route for zhihu"

	// /api/v1/author/zhihu
	zhihuAuthorApi := apiGroup.GET("/author/zhihu/:id", zhihuHandler.AuthorName)
	zhihuAuthorApi.Name = "Author name route for zhihu"

	// /api/v1/db/zhihu
	zhihuDBApi := apiGroup.Group("/db/zhihu")
	zhihuDBAdd := zhihuDBApi.POST("/add", zhihuHandler.Add)
	zhihuDBAdd.Name = "Add route for zhihu db api"
	zhihuDBUpdate := zhihuDBApi.POST("/update", zhihuHandler.Update)
	zhihuDBUpdate.Name = "Update route for zhihu db api"
	zhihuDBDelete := zhihuDBApi.DELETE("/:id", zhihuHandler.Delete)
	zhihuDBDelete.Name = "Delete route for zhihu db api"
	zhihuDBList := zhihuDBApi.GET("", zhihuHandler.List)
	zhihuDBList.Name = "List route for zhihu db api"
	zhihuDBActivate := zhihuDBApi.POST("/activate/:id", zhihuHandler.Activate)
	zhihuDBActivate.Name = "Activate route for zhihu db api"

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

	archiveHandler := archiveController.NewController(db)
	// /api/v1/archive
	archiveApi := apiGroup.Group("/archive")
	archivePickApi := archiveApi.GET("", archiveHandler.Archive)
	archivePickApi.Name = "Archive pick route"
	archiveHistoryApi := archiveApi.GET("/:url", archiveHandler.History)
	archiveHistoryApi.Name = "Archive route"
	randomPickApi := archiveApi.POST("/random", archiveHandler.Random)
	randomPickApi.Name = "Random pick route"
	selectPickApi := archiveApi.POST("/select", archiveHandler.Select)
	selectPickApi.Name = "Select pick route"

	// /api/v1/job
	jobApi := apiGroup.Group("/job")
	startJobApi := jobApi.POST("/start/:task", jobHandler.StartJob)
	startJobApi.Name = "Start job route"
	getJobsApi := jobApi.GET("/list", jobHandler.GetJobs)
	getJobsApi.Name = "Get jobs route"
	getErrorJobsApi := jobApi.GET("/list/error", jobHandler.GetErrorJobs)
	getErrorJobsApi.Name = "Get error jobs route"
	addTaskApi := jobApi.POST("/task", jobHandler.AddTask)
	addTaskApi.Name = "Add task route"
	patchTaskApi := jobApi.POST("/task/patch", jobHandler.PatchTask)
	patchTaskApi.Name = "Patch task route"
	deleteTaskApi := jobApi.DELETE("/task/:id", jobHandler.DeleteTask)
	deleteTaskApi.Name = "Delete task route"

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

// setupCronCrawlJob sets up cron jobs
func setupCronCrawlJob(logger *zap.Logger,
	redisService redis.Redis,
	db *gorm.DB,
	notifier notify.Notifier,
) (definitionToFunc jobController.DefinitionToFunc, err error) {
	cronService, err := cron.NewCronService(logger)
	if err != nil {
		return nil, fmt.Errorf("cron service init failed: %w", err)
	}

	cronDBService := cronDB.NewDBService(db)
	runningJobs, err := cronDBService.FindRunningJob()
	if err != nil {
		return nil, fmt.Errorf("failed to find running cron jobs: %w", err)
	}

	for _, job := range runningJobs {
		definition, err := cronDBService.GetDefinition(job.TaskType)
		if err != nil {
			return nil, fmt.Errorf("failed to get cron task definition: %w", err)
		}

		switch definition.Type {
		case cronDB.TypeZsxq:
			crawlFunc := cron.GenerateRealCrawlFunc(zsxqCron.Crawl(job.ID, definition.ID, definition.Include, definition.Exclude, job.Detail, redisService, db, notifier))
			go crawlFunc()
			logger.Info("Start zsxq running job", zap.String("job_id", job.ID))
		case cronDB.TypeZhihu:
			crawlFunc := cron.GenerateRealCrawlFunc(zhihuCron.Crawl(job.ID, definition.ID, definition.Include, definition.Exclude, job.Detail, redisService, db, notifier))
			go crawlFunc()
			logger.Info("Start zhihu running job", zap.String("job_id", job.ID))
		case cronDB.TypeXiaobot:
			// Xiaobot crawl is quick and simple, so do not need to resume running job
			if err = cronDBService.UpdateStatus(job.ID, cronDB.StatusStopped); err != nil {
				return nil, fmt.Errorf("failed to stop xiaobot running job: %w", err)
			}
		default:
			return nil, fmt.Errorf("unknown cron job type %d", definition.Type)
		}
	}

	definitions, err := cronDBService.GetDefinitions()
	if err != nil {
		return nil, fmt.Errorf("failed to get cron task definitions: %w", err)
	}

	definitionToFunc = make(jobController.DefinitionToFunc)

	for _, definition := range definitions {
		var jobID string
		var crawlFunc jobController.CrawlFunc

		switch definition.Type {
		case cronDB.TypeZsxq:
			crawlFunc = zsxqCron.Crawl("", definition.ID, definition.Include, definition.Exclude, "", redisService, db, notifier)
			if jobID, err = cronService.AddCrawlJob("zsxq_crawl", definition.CronExpr, crawlFunc); err != nil {
				return nil, fmt.Errorf("failed to add zsxq cron job: %w", err)
			}
			logger.Info("Add zsxq cron crawl job successfully", zap.String("job_id", jobID))
			if err = cronDBService.PatchDefinition(definition.ID, nil, nil, nil, &jobID); err != nil {
				return nil, fmt.Errorf("failed to patch cron task definition: %w", err)
			}
		case cronDB.TypeZhihu:
			crawlFunc = zhihuCron.Crawl("", definition.ID, definition.Include, definition.Exclude, "", redisService, db, notifier)
			if jobID, err = cronService.AddCrawlJob("zhihu_crawl", definition.CronExpr, crawlFunc); err != nil {
				return nil, fmt.Errorf("failed to add zhihu cron job: %w", err)
			}
			logger.Info("Add zhihu cron crawl job successfully", zap.String("job_id", jobID))
			if err = cronDBService.PatchDefinition(definition.ID, nil, nil, nil, &jobID); err != nil {
				return nil, fmt.Errorf("failed to patch cron task definition: %w", err)
			}
		case cronDB.TypeXiaobot:
			crawlFunc = xiaobotCron.Crawl(redisService, db, notifier)
			if jobID, err = cronService.AddCrawlJob("xiaobot_crawl", definition.CronExpr, crawlFunc); err != nil {
				return nil, fmt.Errorf("failed to add xiaobot cron job: %w", err)
			}
			logger.Info("Add xiaobot cron crawl job successfully", zap.String("job_id", jobID))
			if err = cronDBService.PatchDefinition(definition.ID, nil, nil, nil, &jobID); err != nil {
				return nil, fmt.Errorf("failed to patch cron task definition: %w", err)
			}
		default:
			return nil, fmt.Errorf("unknown cron job type %d", definition.Type)
		}

		definitionToFunc[definition.ID] = crawlFunc
	}

	return definitionToFunc, nil
}
