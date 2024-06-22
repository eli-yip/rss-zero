package crawler

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/log"
	notify "github.com/eli-yip/rss-zero/internal/notify"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zhihuRequest "github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

func TestPinList(t *testing.T) {
	u := `https://www.zhihu.com/api/v4/members/daleige/pins`
	u = fmt.Sprintf("%s?%s", u, "offset=0&limit=20")
	assert := assert.New(t)
	assert.Nil(config.InitForTestToml())

	db, err := db.NewPostgresDB(config.C.Database)
	assert.Nil(err)
	zhihuDBService := zhihuDB.NewDBService(db)
	requestService, err := zhihuRequest.NewRequestService(log.NewZapLogger(), zhihuDBService, notify.NewBarkNotifier(config.C.Bark.URL), "")
	assert.Nil(err)
	logger := log.NewZapLogger()
	defer requestService.ClearCache(logger)
	bytes, err := requestService.LimitRaw(u, logger)
	assert.Nil(err)

	path := filepath.Join("examples", "pin_list.json")
	dir := filepath.Dir(path)
	err = os.MkdirAll(dir, 0755)
	assert.Nil(err)

	_, err = os.Stat(path)
	if err == nil {
		t.Log("File already exists. Skipping write.")
		return
	}

	if os.IsNotExist(err) {
		err = os.WriteFile(path, bytes, 0644)
		assert.Nil(err)
	} else {
		t.Fatal(err)
	}
}
