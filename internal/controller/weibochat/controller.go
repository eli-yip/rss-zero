package weibochat

import (
	"slices"

	"github.com/eli-yip/rss-zero/internal/weibochat"
)

type Controller struct {
	store    *weibochat.PostgresStore
	groupIDs []string
}

func NewController(store *weibochat.PostgresStore, groupIDs []string) *Controller {
	return &Controller{store: store, groupIDs: slices.Clone(groupIDs)}
}
