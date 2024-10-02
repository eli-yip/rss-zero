package crawl

import (
	"testing"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/migrate"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"github.com/stretchr/testify/assert"
)

func TestCrawlZvideo(t *testing.T) {
	assert := assert.New(t)

	config.InitForTestToml()

	const user = `canglimo`

	logger := log.NewZapLogger()

	db, err := db.NewPostgresDB(config.C.Database)
	assert.Nil(err)
	err = migrate.MigrateDB(db)
	assert.Nil(err)

	cookieService := cookie.NewCookieService(db)
	d_c0, z_c0, zse_ck, err := cookie.GetCookies(cookieService, logger)
	assert.Nil(err)
	zhihuDBService := zhihuDB.NewDBService(db)
	notifier := notify.NewBarkNotifier(config.C.Bark.URL)

	requestService, err := request.NewRequestService(logger, zhihuDBService, notifier, request.Cookie{DC0: d_c0, ZseCk: zse_ck, ZC0: z_c0}, request.WithLimiter(request.TokenCh))
	assert.Nil(err)

	parser := parse.NewZvideoParseService(zhihuDBService)

	err = CrawlZvideo(user, requestService, parser, notifier, 0, true, logger)
	assert.Nil(err)
}
