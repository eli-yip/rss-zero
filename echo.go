package main

import (
	"net"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"gorm.io/gorm"

	archiveController "github.com/eli-yip/rss-zero/internal/controller/archive"
	endoflifeController "github.com/eli-yip/rss-zero/internal/controller/endoflife"
	githubController "github.com/eli-yip/rss-zero/internal/controller/github"
	jobController "github.com/eli-yip/rss-zero/internal/controller/job"
	mackedController "github.com/eli-yip/rss-zero/internal/controller/macked"
	migrateController "github.com/eli-yip/rss-zero/internal/controller/migrate"
	rsshubController "github.com/eli-yip/rss-zero/internal/controller/rsshub"
	xiaobotController "github.com/eli-yip/rss-zero/internal/controller/xiaobot"
	zhihuController "github.com/eli-yip/rss-zero/internal/controller/zhihu"
	zsxqController "github.com/eli-yip/rss-zero/internal/controller/zsxq"
	myMiddleware "github.com/eli-yip/rss-zero/internal/middleware"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	githubDB "github.com/eli-yip/rss-zero/pkg/routers/github/db"
	"github.com/eli-yip/rss-zero/pkg/routers/macked"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

func setupEcho(redisService redis.Redis, cookieService cookie.CookieIface, db *gorm.DB, tg macked.BotIface, notifier notify.Notifier,
	definitionToFunc jobController.DefinitionToFunc,
	cronService *cron.CronService, logger *zap.Logger,
) (e *echo.Echo) {
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
		middleware.RequestID(), // add request id
		middleware.Recover(),   // recover from panic
		middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowHeaders: []string{"*"},
			AllowMethods: []string{"*"},
		}),
		myMiddleware.LogRequest(logger),   // log request
		myMiddleware.InjectLogger(logger), // inject logger to context
	)

	zsxqHandler := zsxqController.NewZsxqController(redisService, cookieService, db, notifier, logger)
	zhihuDBService := zhihuDB.NewDBService(db)
	zhihuHandler := zhihuController.NewController(redisService, cookieService, zhihuDBService, notifier)
	xiaobotDBService := xiaobotDB.NewDBService(db)
	xiaobotHandler := xiaobotController.NewController(redisService, cookieService, xiaobotDBService, notifier, logger)
	endOfLifeHandler := endoflifeController.NewController(redisService, logger)
	cronDBService := cronDB.NewDBService(db)
	jobHandler := jobController.NewController(cronService,
		redisService, cookieService, db, notifier,
		cronDBService, definitionToFunc, logger)
	archiveHandler := archiveController.NewController(db)
	githubDBService := githubDB.NewDBService(db)
	githubController := githubController.NewController(redisService, cookieService, githubDBService, notifier)
	mackedController := mackedController.NewController(redisService, tg, macked.NewDBService(db), logger)

	migrateHandler := migrateController.NewController(logger, db)

	registerRSS(e, zsxqHandler, zhihuHandler, xiaobotHandler, endOfLifeHandler, githubController, mackedController)

	// /api/v1
	apiGroup := e.Group("/api/v1")
	registerFeed(apiGroup, zhihuHandler, githubController)
	registerCookie(apiGroup, zsxqHandler, xiaobotHandler, zhihuHandler, githubController)
	registerAuthor(apiGroup, zhihuHandler)
	registerDEncryptionService(apiGroup, zhihuHandler)
	registerReformat(apiGroup, zsxqHandler, zhihuHandler, xiaobotHandler)
	registerExport(apiGroup, zsxqHandler, zhihuHandler, xiaobotHandler)
	registerArchive(apiGroup, archiveHandler)
	registerJob(apiGroup, jobHandler)
	registerSub(apiGroup, zhihuHandler, githubController, xiaobotHandler)
	registerMigrate(apiGroup, migrateHandler)

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

func registerAuthor(apiGroup *echo.Group, zhihuHandler *zhihuController.Controller) {
	// /api/v1/author/zhihu
	zhihuAuthorApi := apiGroup.GET("/author/zhihu/:id", zhihuHandler.AuthorName)
	zhihuAuthorApi.Name = "Author name route for zhihu"
}

// /api/v1/feed
// /api/v1/feed/zhihu/:id
// /api/v1/feed/rsshub
func registerFeed(apiGroup *echo.Group, zhihuHandler *zhihuController.Controller, githubController *githubController.Controller) {
	feedApi := apiGroup.Group("/feed")

	zhihuFeedApi := feedApi.GET("/zhihu/:id", zhihuHandler.Feed)
	zhihuFeedApi.Name = "Feed route for zhihu"
	rssHubFeddApi := feedApi.POST("/rsshub", rsshubController.GenerateRSSHubFeed)
	rssHubFeddApi.Name = "RSSHub feed generator route"

	githubFeedApi := feedApi.GET("/github/:user_repo", githubController.Feed)
	githubFeedApi.Name = "Feed route for github"
}

// /api/v1/job
func registerJob(apiGroup *echo.Group, jobHandler *jobController.Controller) {
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
	listTaskApi := jobApi.GET("/task/list", jobHandler.ListTask)
	listTaskApi.Name = "List task route"
}

// /api/v1/archive
func registerArchive(apiGroup *echo.Group, archiveHandler *archiveController.Controller) {
	archiveApi := apiGroup.Group("/archive")
	archivePickApi := archiveApi.GET("", archiveHandler.Archive)
	archivePickApi.Name = "Archive pick route"
	archiveHistoryApi := archiveApi.GET("/:url", archiveHandler.History)
	archiveHistoryApi.Name = "Archive route"
	randomPickApi := archiveApi.POST("/random", archiveHandler.Random)
	randomPickApi.Name = "Random pick route"
	selectPickApi := archiveApi.POST("/select", archiveHandler.Select)
	selectPickApi.Name = "Select pick route"
}

// /api/v1/export
// /api/v1/export/zsxq
// /api/v1/export/zhihu
// /api/v1/export/xiaobot
func registerExport(apiGroup *echo.Group, zsxqHandler *zsxqController.Controoler, zhihuHandler *zhihuController.Controller, xiaobotHandler *xiaobotController.Controller) {
	exportApi := apiGroup.Group("/export")

	exportZsxqApi := exportApi.POST("/zsxq", zsxqHandler.Export)
	exportZsxqApi.Name = "Export route for zsxq"

	exportZhihuApi := exportApi.POST("/zhihu", zhihuHandler.Export)
	exportZhihuApi.Name = "Export route for zhihu"

	exportXiaobotApi := exportApi.POST("/xiaobot", xiaobotHandler.Export)
	exportXiaobotApi.Name = "Export route for xiaobot"
}

// /api/v1/es
func registerDEncryptionService(apiGroup *echo.Group, zhihuHandler *zhihuController.Controller) {
	zhihuEncryptionServiceApi := apiGroup.Group("/es/zhihu")
	zhihuEncryptionServiceAdd := zhihuEncryptionServiceApi.POST("/add", zhihuHandler.Add)
	zhihuEncryptionServiceAdd.Name = "Add route for zhihu db api"
	zhihuEncryptionServiceUpdate := zhihuEncryptionServiceApi.POST("/update", zhihuHandler.Update)
	zhihuEncryptionServiceUpdate.Name = "Update route for zhihu db api"
	zhihuEncryptionServiceDelete := zhihuEncryptionServiceApi.DELETE("/:id", zhihuHandler.Delete)
	zhihuEncryptionServiceDelete.Name = "Delete route for zhihu db api"
	zhihuEncryptionServiceList := zhihuEncryptionServiceApi.GET("", zhihuHandler.List)
	zhihuEncryptionServiceList.Name = "List route for zhihu db api"
	zhihuEncryptionServiceActivate := zhihuEncryptionServiceApi.POST("/activate/:id", zhihuHandler.Activate)
	zhihuEncryptionServiceActivate.Name = "Activate route for zhihu db api"
}

// /api/v1/refmt
// /api/v1/refmt/zsxq
// /api/v1/refmt/zhihu
// /api/v1/refmt/xiaobot
func registerReformat(apiGroup *echo.Group, zsxqHandler *zsxqController.Controoler, zhihuHandler *zhihuController.Controller, xiaobotHandler *xiaobotController.Controller) {
	refmtApi := apiGroup.Group("/refmt")

	refmtZsxqApi := refmtApi.POST("/zsxq", zsxqHandler.Reformat)
	refmtZsxqApi.Name = "Reformat route for zsxq"

	refmtZhihuApi := refmtApi.POST("/zhihu", zhihuHandler.Reformat)
	refmtZhihuApi.Name = "Reformat route for zhihu"

	refmtXiaobotApi := refmtApi.POST("/xiaobot", xiaobotHandler.Reformat)
	refmtXiaobotApi.Name = "Reformat route for xiaobot"
}

// /api/v1/cookie
// /api/v1/cookie/zsxq
// /api/v1/cookie/xiaobot
// /api/v1/cookie/zhihu
// /api/v1/cookie/zhihu/check
func registerCookie(apiGroup *echo.Group, zsxqHandler *zsxqController.Controoler, xiaobotHandler *xiaobotController.Controller, zhihuHandler *zhihuController.Controller, githubController *githubController.Controller) {
	cookieApi := apiGroup.Group("/cookie")

	zsxqCookieApi := cookieApi.POST("/zsxq", zsxqHandler.UpdateCookie)
	zsxqCookieApi.Name = "Cookie updating route for zsxq"

	xiaobotCookieApi := cookieApi.POST("/xiaobot", xiaobotHandler.UpdateToken)
	xiaobotCookieApi.Name = "Token updating route for xiaobot"

	zhihuCookieApi := cookieApi.POST("/zhihu", zhihuHandler.UpdateCookie)
	zhihuCookieApi.Name = "Cookie updating route for zhihu"

	zhihuCheckCookieApi := cookieApi.GET("/zhihu", zhihuHandler.CheckCookie)
	zhihuCheckCookieApi.Name = "Cookie checking route for zhihu"

	githubCookieApi := cookieApi.POST("/github", githubController.UpdateToken)
	githubCookieApi.Name = "Token updating route for github"
}

// /rss
func registerRSS(e *echo.Echo, zsxqHandler *zsxqController.Controoler, zhihuHandler *zhihuController.Controller, xiaobotHandler *xiaobotController.Controller, endOfLifeHandler *endoflifeController.Controller, githubController *githubController.Controller, mackedController *mackedController.Controller) {
	rssGroup := e.Group("/rss")
	rssGroup.Use(
		myMiddleware.SetRSSContentType(), // set content type to application/atom+xml
		myMiddleware.ExtractFeedID(),     // extract feed id from url and set it to context
	)

	rssZsxq := rssGroup.GET("/zsxq/:feed", zsxqHandler.RSS)
	rssZsxq.Name = "RSS route for zsxq group"

	rssZsxqRandom := rssGroup.GET("/zsxq/random", zsxqHandler.RandomCanglimoDigest)
	rssZsxqRandom.Name = "RSS route for zsxq random canglimo digest"

	rssZhihu := rssGroup.Group("/zhihu")

	rssZhihuAnswer := rssZhihu.GET("/answer/:feed", zhihuHandler.AnswerRSS)
	rssZhihuAnswer.Name = "RSS route for zhihu answer"

	rssZhihuArticle := rssZhihu.GET("/article/:feed", zhihuHandler.ArticleRSS)
	rssZhihuArticle.Name = "RSS route for zhihu article"

	rssZhihuPin := rssZhihu.GET("/pin/:feed", zhihuHandler.PinRSS)
	rssZhihuPin.Name = "RSS route for zhihu pin"

	rssZhihuRandom := rssZhihu.GET("/random", zhihuHandler.RandomCanglimoAnswers)
	rssZhihuRandom.Name = "RSS route for zhihu random canglimo answers"

	rssXiaobot := rssGroup.GET("/xiaobot/:feed", xiaobotHandler.RSS)
	rssXiaobot.Name = "RSS route for xiaobot"

	rssEndOfLife := rssGroup.GET("/endoflife/:feed", endOfLifeHandler.RSS)
	rssEndOfLife.Name = "RSS route for endoflife.date"

	rssMackedBare := rssGroup.GET("/macked", mackedController.RSS)
	rssMackedBare.Name = "RSS bare route for macked"

	// Add :feed here to fit the ExtractFeedID middleware
	rssMacked := rssGroup.GET("/macked/:feed", mackedController.RSS)
	rssMacked.Name = "RSS route for macked"

	rssGithub := rssGroup.GET("/github/:feed", githubController.RSS)
	rssGithub.Name = "RSS route for github"

	githubRSSPreApi := rssGroup.GET("/github/pre/:feed", githubController.RSS)
	githubRSSPreApi.Name = "RSS route for github pre"
}

func registerSub(apiGroup *echo.Group, zhihuHandler *zhihuController.Controller, github *githubController.Controller, xiaobotHandler *xiaobotController.Controller) {
	// /api/v1/sub/zhihu
	zhihuSubApi := apiGroup.GET("/sub/zhihu", zhihuHandler.GetSubs)
	zhihuSubApi.Name = "Sub list route for zhihu"
	zhihuDeleteSubApi := apiGroup.DELETE("/sub/zhihu/:id", zhihuHandler.DeleteSub)
	zhihuDeleteSubApi.Name = "Delete sub route for zhihu"
	zhihuActivateSubApi := apiGroup.POST("/sub/zhihu/activate/:id", zhihuHandler.ActivateSub)
	zhihuActivateSubApi.Name = "Activate sub route for zhihu"

	// /api/v1/sub/github
	githubSubApi := apiGroup.GET("/sub/github", github.GetSubs)
	githubSubApi.Name = "Sub list route for github"
	githubDeleteSubApi := apiGroup.DELETE("/sub/github/:id", github.DeleteSub)
	githubDeleteSubApi.Name = "Delete sub route for github"
	githubActivateSubApi := apiGroup.POST("/sub/github/activate/:id", github.ActivateSub)
	githubActivateSubApi.Name = "Activate sub route for github"

	// /api/v1/sub/xiaobot
	xiaobotSubApi := apiGroup.GET("/sub/xiaobot", xiaobotHandler.GetSubs)
	xiaobotSubApi.Name = "Sub list route for xiaobot"
	xiaobotDeleteSubApi := apiGroup.DELETE("/sub/xiaobot/:id", xiaobotHandler.DeleteSub)
	xiaobotDeleteSubApi.Name = "Delete sub route for xiaobot"
	xiaobotActivateSubApi := apiGroup.POST("/sub/xiaobot/activate/:id", xiaobotHandler.ActivateSub)
	xiaobotActivateSubApi.Name = "Activate sub route for xiaobot"
}

func registerMigrate(apiGroup *echo.Group, migrateHandler *migrateController.Controller) {
	migrateApi := apiGroup.Group("/migrate")
	migrateMinioApi := migrateApi.POST("/20240905", migrateHandler.Migrate)
	migrateMinioApi.Name = "Migrate minio files route 20240905"
}
