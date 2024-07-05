package crawl

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/log"
	notify "github.com/eli-yip/rss-zero/internal/notify"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zhihuRequest "github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"github.com/stretchr/testify/assert"
)

func TestArticleList(t *testing.T) {
	u := `https://www.zhihu.com/api/v4/members/canglimo/articles`
	u = fmt.Sprintf("%s?%s", u, "offset=0&limit=20")
	assert := assert.New(t)
	assert.Nil(config.InitForTestToml())

	logger := log.NewZapLogger()
	db, err := db.NewPostgresDB(config.C.Database)
	assert.Nil(err)
	zhihuDBService := zhihuDB.NewDBService(db)
	requestService, err := zhihuRequest.NewRequestService(logger, zhihuDBService, notify.NewBarkNotifier(config.C.Bark.URL), "")
	if err != nil {
		t.Fatal(err)
	}
	defer requestService.ClearCache(logger)
	bytes, err := requestService.LimitRaw(u, logger)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join("examples", "article_list.json")
	_, err = os.Stat(path)
	if err == nil {
		fmt.Println("File already exists. Skipping write.")
	} else if os.IsNotExist(err) {
		err = os.WriteFile(path, bytes, 0644)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		t.Fatal(err)
	}
}
