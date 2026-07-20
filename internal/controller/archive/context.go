package archive

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/eli-yip/rss-zero/pkg/httputil"
)

func contextUsername(c *echo.Context) (string, error) {
	username, err := echo.ContextGet[string](c, "username")
	if err != nil {
		return "", httputil.NewHTTPError(http.StatusInternalServerError, "missing username")
	}
	return username, nil
}
