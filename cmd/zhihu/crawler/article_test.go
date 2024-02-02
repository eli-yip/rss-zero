package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/log"
	zhihuRequest "github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

func TestArticleList(t *testing.T) {
	u := `https://www.zhihu.com/api/v4/members/canglimo/articles`
	u = fmt.Sprintf("%s?%s", u, "offset=0&limit=20")
	config.InitFromEnv()

	logger := log.NewLogger()
	requestService, err := zhihuRequest.NewRequestService(nil, logger)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := requestService.LimitRaw(u)
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
