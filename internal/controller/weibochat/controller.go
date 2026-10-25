package weibochat

import (
	"net/http"
	"slices"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/labstack/echo/v5"
)

type Controller struct {
	store    *weibochat.PostgresStore
	groupIDs []string
}

func NewController(store *weibochat.PostgresStore, groupIDs []string) *Controller {
	return &Controller{store: store, groupIDs: slices.Clone(groupIDs)}
}

func ownerUsername(c *echo.Context) (string, error) {
	username, err := echo.ContextGet[string](c, "username")
	if err != nil || username == "" {
		return "", httputil.NewHTTPError(http.StatusUnauthorized, "missing username")
	}
	return username, nil
}
