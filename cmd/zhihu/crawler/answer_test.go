package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/log"
	zhihuRequest "github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

func TestAnswerList(t *testing.T) {
	u := `https://www.zhihu.com/api/v4/members/canglimo/answers`
	params := `data[*].is_normal,admin_closed_comment,reward_info,is_collapsed,annotation_action,annotation_detail,collapse_reason,collapsed_by,suggest_edit,comment_count,can_comment,content,voteup_count,reshipment_settings,comment_permission,mark_infos,created_time,updated_time,review_info,question,excerpt,is_labeled,label_info,relationship.is_authorized,voting,is_author,is_thanked,is_nothelp;data[*].author.badge[?(type=best_answerer)].topics`
	escaped := url.QueryEscape(params)
	u = fmt.Sprintf("%s?include=%s&%s", u, escaped, "offset=0&limit=20&sort_by=created")
	config.InitConfigFromEnv()

	logger := log.NewLogger()
	requestService, err := zhihuRequest.NewRequestService(logger)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := requestService.LimitRaw(u)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join("examples", "answer_list.json")
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
