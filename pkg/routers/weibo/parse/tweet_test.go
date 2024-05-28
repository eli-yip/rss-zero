package parse

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/render"
	weiboDB "github.com/eli-yip/rss-zero/pkg/routers/weibo/db"
	"github.com/eli-yip/rss-zero/pkg/routers/weibo/request"
)

func TestParseTweet(t *testing.T) {
	assert := assert.New(t)
	assert.Nil(config.InitForTestToml())

	usersFile := filepath.Join("..", "example", "user.json")
	logger := log.NewZapLogger()
	fileService, err := file.NewFileServiceMinio(config.C.Minio, logger)
	assert.Nil(err)
	dbService, err := db.NewPostgresDB(config.C.Database)
	assert.Nil(err)
	weiboDBService := weiboDB.NewDBService(dbService)
	redisService, err := redis.NewRedisService(config.C.Redis)
	assert.Nil(err)
	cookies := os.Getenv("WEIBO_COOKIES")
	assert.NotEmpty(cookies)
	requestService, err := request.NewRequestService(redisService, cookies, logger)
	assert.Nil(err)
	htmlToMarkdownService := render.NewHTMLToMarkdownService(logger)
	parseService := NewParseService(fileService, requestService, weiboDBService, htmlToMarkdownService, md.NewMarkdownFormatter(), logger)

	data, err := os.ReadFile(usersFile)
	assert.Nil(err)
	list, err := parseService.ParseTweetList(data)
	assert.Nil(err)

	for _, tweet := range list {
		data, err := json.Marshal(tweet)
		assert.Nil(err)
		text, err := parseService.ParseTweet(data)
		assert.Nil(err)
		t.Log(text)
	}
}

func TestParseTime(t *testing.T) {
	assert := assert.New(t)
	assert.Nil(config.InitForTestToml())

	cases := []struct {
		timeStr string
		want    time.Time
	}{
		{"Mon May 06 20:06:59 +0800 2024", time.Date(2024, 5, 6, 20, 6, 59, 0, config.C.BJT)},
		{"Mon May 06 12:46:58 +0800 2024", time.Date(2024, 5, 6, 12, 46, 58, 0, config.C.BJT)},
	}

	for _, c := range cases {
		got, err := parseTime(c.timeStr)
		assert.Nil(err)
		assert.Equal(c.want, got)
	}
}
