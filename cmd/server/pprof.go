package main

import (
	"net/http"
	"net/http/pprof"

	"github.com/labstack/echo/v5"
)

// registerPprof 将 net/http/pprof 注册到 Echo，避免依赖仅支持 Echo v4 的包装库。
func registerPprof(e *echo.Echo) {
	g := e.Group("/debug/pprof")
	g.GET("", wrapHTTPHandler(http.HandlerFunc(pprof.Index)))
	g.GET("/", wrapHTTPHandler(http.HandlerFunc(pprof.Index)))
	g.GET("/cmdline", wrapHTTPHandler(http.HandlerFunc(pprof.Cmdline)))
	g.GET("/profile", wrapHTTPHandler(http.HandlerFunc(pprof.Profile)))
	g.GET("/symbol", wrapHTTPHandler(http.HandlerFunc(pprof.Symbol)))
	g.POST("/symbol", wrapHTTPHandler(http.HandlerFunc(pprof.Symbol)))
	g.GET("/trace", wrapHTTPHandler(http.HandlerFunc(pprof.Trace)))

	for _, profile := range []string{"block", "goroutine", "heap", "mutex", "threadcreate"} {
		g.GET("/"+profile, wrapHTTPHandler(pprof.Handler(profile)))
	}
}

func wrapHTTPHandler(handler http.Handler) echo.HandlerFunc {
	return func(c *echo.Context) error {
		handler.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}
