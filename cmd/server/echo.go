package main

import (
	"net"
	"net/http"
	"slices"

	echopprof "github.com/eli-yip/echo-pprof"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/ai"
	archiveController "github.com/eli-yip/rss-zero/internal/controller/archive"
	endoflifeController "github.com/eli-yip/rss-zero/internal/controller/endoflife"
	githubController "github.com/eli-yip/rss-zero/internal/controller/github"
	jobController "github.com/eli-yip/rss-zero/internal/controller/job"
	mackedHandler "github.com/eli-yip/rss-zero/internal/controller/macked"
	migrateController "github.com/eli-yip/rss-zero/internal/controller/migrate"
	parseHandler "github.com/eli-yip/rss-zero/internal/controller/parse"
	rsshubController "github.com/eli-yip/rss-zero/internal/controller/rsshub"
	userController "github.com/eli-yip/rss-zero/internal/controller/user"
	xiaobotController "github.com/eli-yip/rss-zero/internal/controller/xiaobot"
	zhihuController "github.com/eli-yip/rss-zero/internal/controller/zhihu"
	zsxqController "github.com/eli-yip/rss-zero/internal/controller/zsxq"
	"github.com/eli-yip/rss-zero/internal/file"
	myMiddleware "github.com/eli-yip/rss-zero/internal/middleware"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/version"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/cron"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	githubDB "github.com/eli-yip/rss-zero/pkg/routers/github/db"
	"github.com/eli-yip/rss-zero/pkg/routers/macked"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

func setupEcho(redisService redis.Redis,
	cookieService cookie.CookieIface,
	db *gorm.DB,
	ai ai.AI,
	notifier notify.Notifier,
	fileService file.File,
	definitionToFunc jobController.DefinitionToFunc,
	cronService *cron.CronService,
	logger *zap.Logger,
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
			AllowOrigins: []string{
				"https://mo.darkeli.com",
				"https://rss-zero.darkeli.com",
				"http://mo.darkeli.com",
				"http://rss-zero.darkeli.com",
				"http://localhost:8080",
				"http://localhost:5173",
			},
			AllowHeaders: []string{
				"content-type",
				"origin",
				"Sec-GPC",
				"Sec-Fetch-Site",
				"Sec-Fetch-Mode",
				"Sec-Fetch-Dest",
			},
			AllowMethods: []string{
				http.MethodGet,
				http.MethodHead,
				http.MethodPut,
				http.MethodPatch,
				http.MethodPost,
				http.MethodDelete,
				http.MethodOptions,
			},
			AllowCredentials: true,
			MaxAge:           60 * 60 * 24,
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
		redisService, cookieService, db, ai, notifier,
		cronDBService, definitionToFunc, logger)
	archiveHandler := archiveController.NewController(db)
	githubDBService := githubDB.NewDBService(db)
	githubController := githubController.NewController(redisService, cookieService, githubDBService, notifier)
	mHandler := mackedHandler.NewHandler(redisService, macked.NewDBService(db), logger)
	parseHandler := parseHandler.NewHandler(db, ai, cookieService, fileService, notifier)
	migrateHandler := migrateController.NewController(logger, db)

	registerRSS(e, zsxqHandler, zhihuHandler, xiaobotHandler, endOfLifeHandler, githubController, mHandler)
	// /api/v1
	apiGroup := e.Group("/api/v1")
	registerArchive(apiGroup, archiveHandler)
	apiGroup.GET("/user", userController.GetUserInfo, myMiddleware.InjectUser())
	bookmarkGroup := apiGroup.Group("/bookmark")
	bookmarkGroup.Use(myMiddleware.InjectUser())
	registerBookmark(bookmarkGroup, archiveHandler)
	tagGroup := apiGroup.Group("/tag")
	tagGroup.Use(myMiddleware.InjectUser())
	registerTag(tagGroup, archiveHandler)

	var groupNeedAuth []*echo.Group

	authorApi := apiGroup.Group("/author")
	groupNeedAuth = append(groupNeedAuth, authorApi)
	registerAuthor(authorApi, zhihuHandler)

	feedApi := apiGroup.Group("/feed")
	groupNeedAuth = append(groupNeedAuth, feedApi)
	registerFeed(feedApi, zhihuHandler, githubController)

	jobApi := apiGroup.Group("/job")
	groupNeedAuth = append(groupNeedAuth, jobApi)
	registerJob(jobApi, jobHandler)

	cookieApi := apiGroup.Group("/cookie")
	groupNeedAuth = append(groupNeedAuth, cookieApi)
	registerCookie(cookieApi, zsxqHandler, xiaobotHandler, zhihuHandler, githubController)

	encryptionServiceApi := apiGroup.Group("/es")
	groupNeedAuth = append(groupNeedAuth, encryptionServiceApi)
	registerDEncryptionService(encryptionServiceApi, zhihuHandler)

	refmtGroup := apiGroup.Group("/refmt")
	groupNeedAuth = append(groupNeedAuth, refmtGroup)
	registerReformat(refmtGroup, zsxqHandler, zhihuHandler, xiaobotHandler)

	exportGroup := apiGroup.Group("/export")
	groupNeedAuth = append(groupNeedAuth, exportGroup)
	registerExport(exportGroup, zsxqHandler, zhihuHandler, xiaobotHandler)

	subGroup := apiGroup.Group("/sub")
	groupNeedAuth = append(groupNeedAuth, subGroup)
	registerSub(subGroup, zhihuHandler, githubController, xiaobotHandler)

	migrateGroup := apiGroup.Group("/migrate")
	groupNeedAuth = append(groupNeedAuth, migrateGroup)
	registerMigrate(migrateGroup, migrateHandler)

	parseGroup := apiGroup.Group("/parse")
	groupNeedAuth = append(groupNeedAuth, parseGroup)
	registerParse(parseGroup, parseHandler)

	mackedGroup := apiGroup.Group("/macked")
	registerMacked(mackedGroup, mHandler)

	for g := range slices.Values(groupNeedAuth) {
		g.Use(myMiddleware.AllowAdmin())
	}

	healthEndpoint := apiGroup.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status":  "ok",
			"version": version.Version,
		})
	})
	healthEndpoint.Name = "Health check route"

	// iterate all routes and log them
	for _, r := range e.Routes() {
		logger.Info("route",
			zap.String("name", r.Name),
			zap.String("path", r.Path))
	}

	echopprof.Wrap(e)

	return e
}

func registerBookmark(bookmarkGroup *echo.Group, archiveHandler *archiveController.Controller) {
	bookmarkList := bookmarkGroup.POST("", archiveHandler.GetBookmarkList)
	bookmarkList.Name = "Bookmark list route"
	bookmarkAdd := bookmarkGroup.PUT("", archiveHandler.PutBookmark)
	bookmarkAdd.Name = "Bookmark add route"
	bookmarkDelete := bookmarkGroup.DELETE("/:id", archiveHandler.DeleteBookmark)
	bookmarkDelete.Name = "Bookmark delete route"
	bookmarkUpdate := bookmarkGroup.PATCH("/:id", archiveHandler.PatchBookmark)
	bookmarkUpdate.Name = "Bookmark update route"
}

func registerTag(tagGroup *echo.Group, archiveHandler *archiveController.Controller) {
	tagList := tagGroup.GET("", archiveHandler.GetAllTags)
	tagList.Name = "Tag list route"
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
	zhihuFeedApi := apiGroup.GET("/zhihu/:id", zhihuHandler.Feed)
	zhihuFeedApi.Name = "Feed route for zhihu"
	rssHubFeddApi := apiGroup.POST("/rsshub", rsshubController.GenerateRSSHubFeed)
	rssHubFeddApi.Name = "RSSHub feed generator route"

	githubFeedApi := apiGroup.GET("/github/:user_repo", githubController.Feed)
	githubFeedApi.Name = "Feed route for github"
}

// /api/v1/job
func registerJob(apiGroup *echo.Group, jobHandler *jobController.Controller) {
	startJobApi := apiGroup.POST("/start/:task", jobHandler.StartJob)
	startJobApi.Name = "Start job route"
	getJobsApi := apiGroup.GET("/list", jobHandler.GetJobs)
	getJobsApi.Name = "Get jobs route"
	getErrorJobsApi := apiGroup.GET("/list/error", jobHandler.GetErrorJobs)
	getErrorJobsApi.Name = "Get error jobs route"
	addTaskApi := apiGroup.POST("/task", jobHandler.AddTask)
	addTaskApi.Name = "Add task route"
	patchTaskApi := apiGroup.POST("/task/patch", jobHandler.PatchTask)
	patchTaskApi.Name = "Patch task route"
	deleteTaskApi := apiGroup.DELETE("/task/:id", jobHandler.DeleteTask)
	deleteTaskApi.Name = "Delete task route"
	listTaskApi := apiGroup.GET("/task/list", jobHandler.ListTask)
	listTaskApi.Name = "List task route"
	runNowApi := apiGroup.POST("/run/:job", jobHandler.RunJobByName)
	runNowApi.Name = "Run job now route"
}

// /api/v1/archive
func registerArchive(apiGroup *echo.Group, archiveHandler *archiveController.Controller) {
	archiveGroup := apiGroup.Group("/archive")
	archiveGroup.Use(myMiddleware.InjectUser())

	archivePickApi := archiveGroup.POST("", archiveHandler.Archive)
	statisticApi := archiveGroup.GET("/statistics", archiveHandler.GetStatistics)
	statisticApi.Name = "Statistics route"
	archivePickApi.Name = "Archive pick route"
	archiveHistoryApi := archiveGroup.GET("/:url", archiveHandler.History)
	archiveHistoryApi.Name = "Archive route"
	randomPickApi := archiveGroup.POST("/random", archiveHandler.Random)
	randomPickApi.Name = "Random pick route"
	zvideoListApi := archiveGroup.GET("/zvideo", archiveHandler.ZvideoList)
	zvideoListApi.Name = "Zvideo list route"
	similarityApi := archiveGroup.GET("/similarity/:id", archiveHandler.Similarity)
	similarityApi.Name = "Similarity route"
	// selectPickApi := archiveApi.POST("/select", archiveHandler.Select)
	// selectPickApi.Name = "Select pick route"
}

// /api/v1/export
// /api/v1/export/zsxq
// /api/v1/export/zhihu
// /api/v1/export/xiaobot
func registerExport(exportApi *echo.Group, zsxqHandler *zsxqController.Controller, zhihuHandler *zhihuController.Controller, xiaobotHandler *xiaobotController.Controller) {
	exportZsxqApi := exportApi.POST("/zsxq", zsxqHandler.Export)
	exportZsxqApi.Name = "Export route for zsxq"

	exportZhihuApi := exportApi.POST("/zhihu", zhihuHandler.Export)
	exportZhihuApi.Name = "Export route for zhihu"

	exportXiaobotApi := exportApi.POST("/xiaobot", xiaobotHandler.Export)
	exportXiaobotApi.Name = "Export route for xiaobot"
}

// /api/v1/es
func registerDEncryptionService(apiGroup *echo.Group, zhihuHandler *zhihuController.Controller) {
	zhihuEncryptionServiceApi := apiGroup.Group("/zhihu")

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
func registerReformat(refmtApi *echo.Group, zsxqHandler *zsxqController.Controller, zhihuHandler *zhihuController.Controller, xiaobotHandler *xiaobotController.Controller) {
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
func registerCookie(apiGroup *echo.Group, zsxqHandler *zsxqController.Controller, xiaobotHandler *xiaobotController.Controller, zhihuHandler *zhihuController.Controller, githubController *githubController.Controller) {
	zsxqCookieApi := apiGroup.POST("/zsxq", zsxqHandler.UpdateCookie)
	zsxqCookieApi.Name = "Cookie updating route for zsxq"

	zsxqCheckCookieApi := apiGroup.GET("/zsxq", zsxqHandler.CheckCookie)
	zsxqCheckCookieApi.Name = "Cookie checking route for zsxq"

	xiaobotCookieApi := apiGroup.POST("/xiaobot", xiaobotHandler.UpdateToken)
	xiaobotCookieApi.Name = "Token updating route for xiaobot"

	zhihuCookieApi := apiGroup.POST("/zhihu", zhihuHandler.UpdateCookie)
	zhihuCookieApi.Name = "Cookie updating route for zhihu"

	zhihuCheckCookieApi := apiGroup.GET("/zhihu", zhihuHandler.CheckCookie)
	zhihuCheckCookieApi.Name = "Cookie checking route for zhihu"

	githubCookieApi := apiGroup.POST("/github", githubController.UpdateToken)
	githubCookieApi.Name = "Token updating route for github"
}

// /rss
func registerRSS(e *echo.Echo, zsxqHandler *zsxqController.Controller, zhihuHandler *zhihuController.Controller, xiaobotHandler *xiaobotController.Controller, endOfLifeHandler *endoflifeController.Controller, githubController *githubController.Controller, mackedController *mackedHandler.Handler) {
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

func registerSub(subApi *echo.Group, zhihuHandler *zhihuController.Controller, github *githubController.Controller, xiaobotHandler *xiaobotController.Controller) {
	// /api/v1/sub/zhihu
	zhihuSubApi := subApi.GET("/zhihu", zhihuHandler.GetSubs)
	zhihuSubApi.Name = "Sub list route for zhihu"
	zhihuDeleteSubApi := subApi.DELETE("/sub/zhihu/:id", zhihuHandler.DeleteSub)
	zhihuDeleteSubApi.Name = "Delete sub route for zhihu"
	zhihuActivateSubApi := subApi.POST("/sub/zhihu/activate/:id", zhihuHandler.ActivateSub)
	zhihuActivateSubApi.Name = "Activate sub route for zhihu"

	// /api/v1/sub/github
	githubSubApi := subApi.GET("/github", github.GetSubs)
	githubSubApi.Name = "Sub list route for github"
	githubDeleteSubApi := subApi.DELETE("/github/:id", github.DeleteSub)
	githubDeleteSubApi.Name = "Delete sub route for github"
	githubActivateSubApi := subApi.POST("/github/activate/:id", github.ActivateSub)
	githubActivateSubApi.Name = "Activate sub route for github"

	// /api/v1/sub/xiaobot
	xiaobotSubApi := subApi.GET("/xiaobot", xiaobotHandler.GetSubs)
	xiaobotSubApi.Name = "Sub list route for xiaobot"
	xiaobotDeleteSubApi := subApi.DELETE("/xiaobot/:id", xiaobotHandler.DeleteSub)
	xiaobotDeleteSubApi.Name = "Delete sub route for xiaobot"
	xiaobotActivateSubApi := subApi.POST("/xiaobot/activate/:id", xiaobotHandler.ActivateSub)
	xiaobotActivateSubApi.Name = "Activate sub route for xiaobot"
}

func registerMigrate(migrateApi *echo.Group, migrateHandler *migrateController.Controller) {
	migrateMinioApi := migrateApi.POST("/20240905", migrateHandler.Migrate20240905)
	migrateMinioApi.Name = "Migrate minio files route 20240905"
	migrate20240929Api := migrateApi.POST("/20240929", migrateHandler.Migrate20240929)
	migrate20240929Api.Name = "Migrate db 20240929 route"
	migrate20250530Api := migrateApi.POST("/20250530", migrateHandler.Migrate20250530)
	migrate20250530Api.Name = "Migrate db 20250530 route"
}

func registerParse(parseApi *echo.Group, parseHandler *parseHandler.Handler) {
	parseZhihuAnswerApi := parseApi.POST("/zhihu/answer", parseHandler.ParseZhihuAnswer)
	parseZhihuAnswerApi.Name = "Parse zhihu answer route"

	parseXiaobotPaperApi := parseApi.POST("/xiaobot", parseHandler.ParseXiaobotPaper)
	parseXiaobotPaperApi.Name = "Parse xiaobot paper route"
}

func registerMacked(mackedApi *echo.Group, mackedHandler *mackedHandler.Handler) {
	mackedAddAppInfoApi := mackedApi.POST("/appinfo", mackedHandler.AddAppInfo)
	mackedAddAppInfoApi.Name = "Add app info route for macked"
}
