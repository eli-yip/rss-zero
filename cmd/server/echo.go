package main

import (
	"net"
	"net/http"

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
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

func setupEcho(redisService redis.Redis, db *gorm.DB, notifier notify.Notifier,
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

	zsxqHandler := zsxqController.NewZsxqHandler(redisService, db, notifier, logger)
	zhihuDBService := zhihuDB.NewDBService(db)
	zhihuHandler := zhihuController.NewZhihuHandler(redisService, zhihuDBService, notifier, logger)
	xiaobotDBService := xiaobotDB.NewDBService(db)
	xiaobotHandler := xiaobotController.NewXiaobotController(redisService, xiaobotDBService, notifier, logger)
	endOfLifeHandler := endoflifeController.NewController(redisService, logger)
	cronDBService := cronDB.NewDBService(db)
	jobHandler := jobController.NewController(cronService,
		redisService, db, notifier,
		cronDBService, definitionToFunc, logger)
	archiveHandler := archiveController.NewController(db)

	registerRSS(e, zsxqHandler, zhihuHandler, xiaobotHandler, endOfLifeHandler)

	// /api/v1
	apiGroup := e.Group("/api/v1")
	registerFeed(apiGroup, zhihuHandler)
	registerCookie(apiGroup, zsxqHandler, xiaobotHandler, zhihuHandler)
	registerAuthor(apiGroup, zhihuHandler)
	registerDBZhihu(apiGroup, zhihuHandler)
	registerReformat(apiGroup, zsxqHandler, zhihuHandler, xiaobotHandler)
	registerExport(apiGroup, zsxqHandler, zhihuHandler, xiaobotHandler)
	registerArchive(apiGroup, archiveHandler)
	registerJob(apiGroup, jobHandler)
	registerRSSHub(apiGroup)

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

// /api/v1/rsshub
func registerRSSHub(apiGroup *echo.Group) {
	rssHubApi := apiGroup.Group("/rsshub")
	feedGeneratorApi := rssHubApi.POST("/feed", rsshubController.GenerateRSSHubFeed)
	feedGeneratorApi.Name = "RSSHub feed generator route"
}

func registerAuthor(apiGroup *echo.Group, zhihuHandler *zhihuController.Controller) {
	// /api/v1/author/zhihu
	zhihuAuthorApi := apiGroup.GET("/author/zhihu/:id", zhihuHandler.AuthorName)
	zhihuAuthorApi.Name = "Author name route for zhihu"
}

// /api/v1/feed
// /api/v1/feed/zhihu/:id
func registerFeed(apiGroup *echo.Group, zhihuHandler *zhihuController.Controller) {
	feedApi := apiGroup.Group("/feed")

	zhihuFeedApi := feedApi.GET("/zhihu/:id", zhihuHandler.Feed)
	zhihuFeedApi.Name = "Feed route for zhihu"
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
func registerExport(apiGroup *echo.Group, zsxqHandler *zsxqController.ZsxqController, zhihuHandler *zhihuController.Controller, xiaobotHandler *xiaobotController.XiaobotController) {
	exportApi := apiGroup.Group("/export")

	exportZsxqApi := exportApi.POST("/zsxq", zsxqHandler.Export)
	exportZsxqApi.Name = "Export route for zsxq"

	exportZhihuApi := exportApi.POST("/zhihu", zhihuHandler.Export)
	exportZhihuApi.Name = "Export route for zhihu"

	exportXiaobotApi := exportApi.POST("/xiaobot", xiaobotHandler.Export)
	exportXiaobotApi.Name = "Export route for xiaobot"
}

// /api/v1/db/zhihu
func registerDBZhihu(apiGroup *echo.Group, zhihuHandler *zhihuController.Controller) {
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
}

// /api/v1/refmt
// /api/v1/refmt/zsxq
// /api/v1/refmt/zhihu
// /api/v1/refmt/xiaobot
func registerReformat(apiGroup *echo.Group, zsxqHandler *zsxqController.ZsxqController, zhihuHandler *zhihuController.Controller, xiaobotHandler *xiaobotController.XiaobotController) {
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
func registerCookie(apiGroup *echo.Group, zsxqHandler *zsxqController.ZsxqController, xiaobotHandler *xiaobotController.XiaobotController, zhihuHandler *zhihuController.Controller) {
	cookieApi := apiGroup.Group("/cookie")

	zsxqCookieApi := cookieApi.POST("/zsxq", zsxqHandler.UpdateZsxqCookie)
	zsxqCookieApi.Name = "Cookie updating route for zsxq"

	xiaobotCookieApi := cookieApi.POST("/xiaobot", xiaobotHandler.UpdateToken)
	xiaobotCookieApi.Name = "Token updating route for xiaobot"

	zhihuCookieApi := cookieApi.POST("/zhihu", zhihuHandler.UpdateCookie)
	zhihuCookieApi.Name = "Cookie updating route for zhihu"

	zhihuCheckCookieApi := cookieApi.GET("/zhihu", zhihuHandler.CheckCookie)
	zhihuCheckCookieApi.Name = "Cookie checking route for zhihu"
}

// /rss
// /rss/zsxq/:feed
// /rss/zhihu/:feed
// /rss/zhihu/answer/:feed
// /rss/zhihu/article/:feed
// /rss/zhihu/pin/:feed
func registerRSS(e *echo.Echo, zsxqHandler *zsxqController.ZsxqController, zhihuHandler *zhihuController.Controller, xiaobotHandler *xiaobotController.XiaobotController, endOfLifeHandler *endoflifeController.Controller) {
	rssGroup := e.Group("/rss")
	rssGroup.Use(
		myMiddleware.SetRSSContentType(), // set content type to application/atom+xml
		myMiddleware.ExtractFeedID(),     // extract feed id from url and set it to context
	)

	rssZsxq := rssGroup.GET("/zsxq/:feed", zsxqHandler.RSS)
	rssZsxq.Name = "RSS route for zsxq group"

	rssZhihu := rssGroup.Group("/zhihu")

	rssZhihuAnswer := rssZhihu.GET("/answer/:feed", zhihuHandler.AnswerRSS)
	rssZhihuAnswer.Name = "RSS route for zhihu answer"

	rssZhihuArticle := rssZhihu.GET("/article/:feed", zhihuHandler.ArticleRSS)
	rssZhihuArticle.Name = "RSS route for zhihu article"

	rssZhihuPin := rssZhihu.GET("/pin/:feed", zhihuHandler.PinRSS)
	rssZhihuPin.Name = "RSS route for zhihu pin"

	rssXiaobot := rssGroup.GET("/xiaobot/:feed", xiaobotHandler.RSS)
	rssXiaobot.Name = "RSS route for xiaobot"

	rssEndOfLife := rssGroup.GET("/endoflife/:feed", endOfLifeHandler.RSS)
	rssEndOfLife.Name = "RSS route for endoflife.date"
}
