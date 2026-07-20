package main

import (
	"context"
	"net"
	"net/http"
	"slices"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/ai"
	archiveController "github.com/eli-yip/rss-zero/internal/controller/archive"
	cookieController "github.com/eli-yip/rss-zero/internal/controller/cookie"
	endoflifeController "github.com/eli-yip/rss-zero/internal/controller/endoflife"
	githubController "github.com/eli-yip/rss-zero/internal/controller/github"
	jobController "github.com/eli-yip/rss-zero/internal/controller/job"
	mackedHandler "github.com/eli-yip/rss-zero/internal/controller/macked"
	migrateController "github.com/eli-yip/rss-zero/internal/controller/migrate"
	parseHandler "github.com/eli-yip/rss-zero/internal/controller/parse"
	rsshubController "github.com/eli-yip/rss-zero/internal/controller/rsshub"
	tkblogHandler "github.com/eli-yip/rss-zero/internal/controller/tkblog"
	tombkeeperHandler "github.com/eli-yip/rss-zero/internal/controller/tombkeeper"
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
	"github.com/eli-yip/rss-zero/pkg/httputil"
	githubDB "github.com/eli-yip/rss-zero/pkg/routers/github/db"
	"github.com/eli-yip/rss-zero/pkg/routers/macked"
	tkblogRouter "github.com/eli-yip/rss-zero/pkg/routers/tkblog"
	tombkeeperRouter "github.com/eli-yip/rss-zero/pkg/routers/tombkeeper"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	xiaobotRequest "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/request"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zsxqRequest "github.com/eli-yip/rss-zero/pkg/routers/zsxq/request"
)

func setupEcho(redisService redis.Redis,
	cookieService cookie.CookieIface,
	db *gorm.DB,
	ai ai.AI,
	notifier notify.Notifier,
	fileService file.File,
	cronService *cron.CronService,
	jobIndex *jobController.JobIndex,
	logger *zap.Logger,
) (e *echo.Echo) {
	e = echo.New()
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
	// Render all errors as the unified {message} envelope.
	e.HTTPErrorHandler = httputil.NewHTTPErrorHandler(logger)

	zsxqHandler := zsxqController.NewZsxqController(redisService, cookieService, db, notifier, logger)
	zhihuDBService := zhihuDB.NewDBService(db)
	zhihuHandler := zhihuController.NewController(redisService, cookieService, zhihuDBService, notifier)
	xiaobotDBService := xiaobotDB.NewDBService(db)
	xiaobotHandler := xiaobotController.NewController(redisService, cookieService, xiaobotDBService, notifier, logger)
	endOfLifeHandler := endoflifeController.NewController(redisService, logger)
	cronDBService := cronDB.NewDBService(db)
	jobHandler := jobController.NewController(cronService, jobIndex,
		redisService, cookieService, db, ai, notifier,
		cronDBService, logger)
	archiveHandler := archiveController.NewController(db)
	githubDBService := githubDB.NewDBService(db)
	githubController := githubController.NewController(redisService, cookieService, githubDBService, notifier)
	cookieHandler := cookieController.NewController(cookieService)
	registerCookieProbes(cookieService)
	mHandler := mackedHandler.NewHandler(redisService, macked.NewDBService(db), logger)
	tombkeeperH := tombkeeperHandler.NewController(redisService, tombkeeperRouter.NewDBService(db), fileService, notifier, logger)
	tkblogH := tkblogHandler.NewController(tkblogRouter.NewDBService(db), notifier, logger)
	parseHandler := parseHandler.NewHandler(db, ai, cookieService, fileService, notifier)
	migrateHandler := migrateController.NewController(logger, db, notifier)

	registerRSS(e, zsxqHandler, zhihuHandler, xiaobotHandler, endOfLifeHandler, githubController, mHandler, tombkeeperH)
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
	registerCookie(cookieApi, cookieHandler)

	encryptionServiceApi := apiGroup.Group("/es")
	groupNeedAuth = append(groupNeedAuth, encryptionServiceApi)
	registerDEncryptionService(encryptionServiceApi, zhihuHandler)

	refmtGroup := apiGroup.Group("/refmt")
	groupNeedAuth = append(groupNeedAuth, refmtGroup)
	registerReformat(refmtGroup, xiaobotHandler)

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

	tombkeeperGroup := apiGroup.Group("/tombkeeper")
	groupNeedAuth = append(groupNeedAuth, tombkeeperGroup)
	registerNamedRoute(tombkeeperGroup, http.MethodPost, "/history", "Tombkeeper history backfill route", tombkeeperH.History)

	tkblogGroup := apiGroup.Group("/tkblog")
	groupNeedAuth = append(groupNeedAuth, tkblogGroup)
	registerNamedRoute(tkblogGroup, http.MethodPost, "/:category/crawl", "Tkblog crawl route", tkblogH.Crawl)

	for g := range slices.Values(groupNeedAuth) {
		g.Use(myMiddleware.AllowAdmin())
	}

	registerNamedRoute(apiGroup, http.MethodGet, "/health", "Health check route", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, httputil.NewResp("ok", map[string]string{
			"status":  "ok",
			"version": version.Version,
		}))
	})

	// iterate all routes and log them
	for _, r := range e.Router().Routes() {
		logger.Info("route",
			zap.String("name", r.Name),
			zap.String("path", r.Path))
	}

	registerPprof(e)

	return e
}

type routeRegistrar interface {
	AddRoute(route echo.Route) (echo.RouteInfo, error)
}

// registerNamedRoute 通过 v5 的 Route API 注册带名称的路由。
func registerNamedRoute(registrar routeRegistrar, method, path, name string, handler echo.HandlerFunc, middleware ...echo.MiddlewareFunc) {
	_, err := registrar.AddRoute(echo.Route{
		Method:      method,
		Path:        path,
		Name:        name,
		Handler:     handler,
		Middlewares: middleware,
	})
	if err != nil {
		panic(err)
	}
}

func registerBookmark(bookmarkGroup *echo.Group, archiveHandler *archiveController.Controller) {
	registerNamedRoute(bookmarkGroup, http.MethodPost, "", "Bookmark list route", archiveHandler.GetBookmarkList)
	registerNamedRoute(bookmarkGroup, http.MethodPut, "", "Bookmark add route", archiveHandler.PutBookmark)
	registerNamedRoute(bookmarkGroup, http.MethodDelete, "/:id", "Bookmark delete route", archiveHandler.DeleteBookmark)
	registerNamedRoute(bookmarkGroup, http.MethodPatch, "/:id", "Bookmark update route", archiveHandler.PatchBookmark)
}

func registerTag(tagGroup *echo.Group, archiveHandler *archiveController.Controller) {
	registerNamedRoute(tagGroup, http.MethodGet, "", "Tag list route", archiveHandler.GetAllTags)
}

func registerAuthor(apiGroup *echo.Group, zhihuHandler *zhihuController.Controller) {
	// /api/v1/author/zhihu
	registerNamedRoute(apiGroup, http.MethodGet, "/author/zhihu/:id", "Author name route for zhihu", zhihuHandler.AuthorName)
}

// /api/v1/feed
// /api/v1/feed/zhihu/:id
// /api/v1/feed/rsshub
func registerFeed(apiGroup *echo.Group, zhihuHandler *zhihuController.Controller, githubController *githubController.Controller) {
	registerNamedRoute(apiGroup, http.MethodGet, "/zhihu/:id", "Feed route for zhihu", zhihuHandler.Feed)
	registerNamedRoute(apiGroup, http.MethodPost, "/rsshub", "RSSHub feed generator route", rsshubController.GenerateRSSHubFeed)

	registerNamedRoute(apiGroup, http.MethodGet, "/github/:user_repo", "Feed route for github", githubController.Feed)
}

// /api/v1/job
func registerJob(apiGroup *echo.Group, jobHandler *jobController.Controller) {
	registerNamedRoute(apiGroup, http.MethodPost, "/start/:task", "Start job route", jobHandler.StartJob)
	registerNamedRoute(apiGroup, http.MethodGet, "/list", "Get jobs route", jobHandler.GetJobs)
	registerNamedRoute(apiGroup, http.MethodGet, "/list/error", "Get error jobs route", jobHandler.GetErrorJobs)
	registerNamedRoute(apiGroup, http.MethodPost, "/task", "Add task route", jobHandler.AddTask)
	registerNamedRoute(apiGroup, http.MethodPost, "/task/patch", "Patch task route", jobHandler.PatchTask)
	registerNamedRoute(apiGroup, http.MethodDelete, "/task/:id", "Delete task route", jobHandler.DeleteTask)
	registerNamedRoute(apiGroup, http.MethodGet, "/task/list", "List task route", jobHandler.ListTask)
	registerNamedRoute(apiGroup, http.MethodPost, "/run/:job", "Run job now route", jobHandler.RunJobByName)
}

// /api/v1/archive
func registerArchive(apiGroup *echo.Group, archiveHandler *archiveController.Controller) {
	archiveGroup := apiGroup.Group("/archive")
	archiveGroup.Use(myMiddleware.InjectUser())

	registerNamedRoute(archiveGroup, http.MethodPost, "", "Archive pick route", archiveHandler.Archive)
	registerNamedRoute(archiveGroup, http.MethodGet, "/statistics", "Statistics route", archiveHandler.GetStatistics)
	registerNamedRoute(archiveGroup, http.MethodGet, "/:url", "Archive route", archiveHandler.History)
	registerNamedRoute(archiveGroup, http.MethodPost, "/random", "Random pick route", archiveHandler.Random)
	registerNamedRoute(archiveGroup, http.MethodGet, "/zvideo", "Zvideo list route", archiveHandler.ZvideoList)
	registerNamedRoute(archiveGroup, http.MethodGet, "/similarity/:id", "Similarity route", archiveHandler.Similarity)
	// registerNamedRoute(archiveGroup, http.MethodPost, "/select", "Select pick route", archiveHandler.Select)
}

// /api/v1/export
// /api/v1/export/zsxq
// /api/v1/export/zhihu
// /api/v1/export/xiaobot
func registerExport(exportApi *echo.Group, zsxqHandler *zsxqController.Controller, zhihuHandler *zhihuController.Controller, xiaobotHandler *xiaobotController.Controller) {
	registerNamedRoute(exportApi, http.MethodPost, "/zsxq", "Export route for zsxq", zsxqHandler.Export)

	registerNamedRoute(exportApi, http.MethodPost, "/zhihu", "Export route for zhihu", zhihuHandler.Export)

	registerNamedRoute(exportApi, http.MethodPost, "/xiaobot", "Export route for xiaobot", xiaobotHandler.Export)
}

// /api/v1/es
func registerDEncryptionService(apiGroup *echo.Group, zhihuHandler *zhihuController.Controller) {
	zhihuEncryptionServiceApi := apiGroup.Group("/zhihu")

	registerNamedRoute(zhihuEncryptionServiceApi, http.MethodPost, "/add", "Add route for zhihu db api", zhihuHandler.Add)
	registerNamedRoute(zhihuEncryptionServiceApi, http.MethodPost, "/update", "Update route for zhihu db api", zhihuHandler.Update)
	registerNamedRoute(zhihuEncryptionServiceApi, http.MethodDelete, "/:id", "Delete route for zhihu db api", zhihuHandler.Delete)
	registerNamedRoute(zhihuEncryptionServiceApi, http.MethodGet, "", "List route for zhihu db api", zhihuHandler.List)
	registerNamedRoute(zhihuEncryptionServiceApi, http.MethodPost, "/activate/:id", "Activate route for zhihu db api", zhihuHandler.Activate)
}

// /api/v1/refmt
// /api/v1/refmt/xiaobot
func registerReformat(refmtApi *echo.Group, xiaobotHandler *xiaobotController.Controller) {
	registerNamedRoute(refmtApi, http.MethodPost, "/xiaobot", "Reformat route for xiaobot", xiaobotHandler.Reformat)
}

// registerCookieProbes wires the optional per-cookie validators into the registry at
// startup. Done here (not in pkg/cookie) because the probes call the platform request
// packages, which themselves import pkg/cookie — embedding them would create a cycle.
func registerCookieProbes(cs cookie.CookieIface) {
	cookie.RegisterProbe(cookie.CookieTypeZsxqAccessToken, func(value string, l *zap.Logger) error {
		_, err := zsxqRequest.NewRequestService(value, l).Limit(context.Background(), config.C.TestURL.Zsxq, l)
		return err
	})
	cookie.RegisterProbe(cookie.CookieTypeXiaobotAccessToken, func(value string, l *zap.Logger) error {
		_, err := xiaobotRequest.NewRequestService(cs, value, l).Limit(config.C.TestURL.Xiaobot)
		return err
	})
}

// /api/v1/cookie  (POST: generic update for any registered cookie, GET: status of all)
func registerCookie(apiGroup *echo.Group, cookieHandler *cookieController.Controller) {
	registerNamedRoute(apiGroup, http.MethodPost, "", "Generic cookie updating route", cookieHandler.UpdateCookies)

	registerNamedRoute(apiGroup, http.MethodGet, "", "Generic cookie status route", cookieHandler.CheckCookies)
}

// /rss
func registerRSS(e *echo.Echo, zsxqHandler *zsxqController.Controller, zhihuHandler *zhihuController.Controller, xiaobotHandler *xiaobotController.Controller, endOfLifeHandler *endoflifeController.Controller, githubController *githubController.Controller, mackedController *mackedHandler.Handler, tombkeeperController *tombkeeperHandler.Controller) {
	rssGroup := e.Group("/rss")
	rssGroup.Use(
		myMiddleware.SetRSSContentType(), // set content type to application/atom+xml
		myMiddleware.ExtractFeedID(),     // extract feed id from url and set it to context
	)

	registerNamedRoute(rssGroup, http.MethodGet, "/zsxq/:feed", "RSS route for zsxq group", zsxqHandler.RSS)

	registerNamedRoute(rssGroup, http.MethodGet, "/zsxq/random", "RSS route for zsxq random canglimo digest", zsxqHandler.RandomCanglimoDigest)

	rssZhihu := rssGroup.Group("/zhihu")

	registerNamedRoute(rssZhihu, http.MethodGet, "/answer/:feed", "RSS route for zhihu answer", zhihuHandler.AnswerRSS)

	registerNamedRoute(rssZhihu, http.MethodGet, "/article/:feed", "RSS route for zhihu article", zhihuHandler.ArticleRSS)

	registerNamedRoute(rssZhihu, http.MethodGet, "/pin/:feed", "RSS route for zhihu pin", zhihuHandler.PinRSS)

	registerNamedRoute(rssZhihu, http.MethodGet, "/random", "RSS route for zhihu random canglimo answers", zhihuHandler.RandomCanglimoAnswers)

	registerNamedRoute(rssGroup, http.MethodGet, "/xiaobot/:feed", "RSS route for xiaobot", xiaobotHandler.RSS)

	registerNamedRoute(rssGroup, http.MethodGet, "/endoflife/:feed", "RSS route for endoflife.date", endOfLifeHandler.RSS)

	registerNamedRoute(rssGroup, http.MethodGet, "/macked", "RSS bare route for macked", mackedController.RSS)

	// Add :feed here to fit the ExtractFeedID middleware
	registerNamedRoute(rssGroup, http.MethodGet, "/macked/:feed", "RSS route for macked", mackedController.RSS)

	registerNamedRoute(rssGroup, http.MethodGet, "/tombkeeper", "RSS bare route for tombkeeper", tombkeeperController.RSS)

	// Add :feed here to fit the ExtractFeedID middleware
	registerNamedRoute(rssGroup, http.MethodGet, "/tombkeeper/:feed", "RSS route for tombkeeper", tombkeeperController.RSS)

	registerNamedRoute(rssGroup, http.MethodGet, "/github/:feed", "RSS route for github", githubController.RSS)

	registerNamedRoute(rssGroup, http.MethodGet, "/github/pre/:feed", "RSS route for github pre", githubController.RSS)
}

func registerSub(subApi *echo.Group, zhihuHandler *zhihuController.Controller, github *githubController.Controller, xiaobotHandler *xiaobotController.Controller) {
	// /api/v1/sub/zhihu
	registerNamedRoute(subApi, http.MethodGet, "/zhihu", "Sub list route for zhihu", zhihuHandler.GetSubs)
	registerNamedRoute(subApi, http.MethodDelete, "/sub/zhihu/:id", "Delete sub route for zhihu", zhihuHandler.DeleteSub)
	registerNamedRoute(subApi, http.MethodPost, "/sub/zhihu/activate/:id", "Activate sub route for zhihu", zhihuHandler.ActivateSub)

	// /api/v1/sub/github
	registerNamedRoute(subApi, http.MethodGet, "/github", "Sub list route for github", github.GetSubs)
	registerNamedRoute(subApi, http.MethodDelete, "/github/:id", "Delete sub route for github", github.DeleteSub)
	registerNamedRoute(subApi, http.MethodPost, "/github/activate/:id", "Activate sub route for github", github.ActivateSub)

	// /api/v1/sub/xiaobot
	registerNamedRoute(subApi, http.MethodGet, "/xiaobot", "Sub list route for xiaobot", xiaobotHandler.GetSubs)
	registerNamedRoute(subApi, http.MethodDelete, "/xiaobot/:id", "Delete sub route for xiaobot", xiaobotHandler.DeleteSub)
	registerNamedRoute(subApi, http.MethodPost, "/xiaobot/activate/:id", "Activate sub route for xiaobot", xiaobotHandler.ActivateSub)
}

func registerMigrate(migrateApi *echo.Group, migrateHandler *migrateController.Controller) {
	registerNamedRoute(migrateApi, http.MethodPost, "/20240905", "Migrate minio files route 20240905", migrateHandler.Migrate20240905)
	registerNamedRoute(migrateApi, http.MethodPost, "/20260612", "Migrate db 20260612 route", migrateHandler.Migrate20260612)

	registerNamedRoute(migrateApi, http.MethodGet, "/registry", "Migration registry status route", migrateHandler.MigrationRegistry)
	registerNamedRoute(migrateApi, http.MethodPost, "/run/:version", "Run migration by version route", migrateHandler.RunMigration)
	registerNamedRoute(migrateApi, http.MethodPost, "/run-pending", "Run pending migrations route", migrateHandler.RunPendingMigrations)
}

func registerParse(parseApi *echo.Group, parseHandler *parseHandler.Handler) {
	registerNamedRoute(parseApi, http.MethodPost, "/zhihu/answer", "Parse zhihu answer route", parseHandler.ParseZhihuAnswer)

	registerNamedRoute(parseApi, http.MethodPost, "/xiaobot", "Parse xiaobot paper route", parseHandler.ParseXiaobotPaper)
}

func registerMacked(mackedApi *echo.Group, mackedHandler *mackedHandler.Handler) {
	registerNamedRoute(mackedApi, http.MethodPost, "/appinfo", "Add app info route for macked", mackedHandler.AddAppInfo)
}
