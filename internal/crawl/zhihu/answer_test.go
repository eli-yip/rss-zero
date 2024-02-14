package crawler

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/log"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	zhihuRequest "github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

func TestAnswerList(t *testing.T) {
	u := `https://www.zhihu.com/api/v4/members/canglimo/answers`
	params := `data[*].is_normal,admin_closed_comment,reward_info,is_collapsed,annotation_action,annotation_detail,collapse_reason,collapsed_by,suggest_edit,comment_count,can_comment,content,voteup_count,reshipment_settings,comment_permission,mark_infos,created_time,updated_time,review_info,question,excerpt,is_labeled,label_info,relationship.is_authorized,voting,is_author,is_thanked,is_nothelp;data[*].author.badge[?(type=best_answerer)].topics`
	escaped := url.QueryEscape(params)
	u = fmt.Sprintf("%s?include=%s&%s", u, escaped, "offset=0&limit=20&sort_by=created")
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

// With this test, we can conclude that not every page is full of answers.
// In the web, it is same. As 192 pages will have 3845 answers, but actually it has 3825 answers.
func TestAnswerListPaging(t *testing.T) {
	config.InitFromEnv()
	logger := log.NewLogger()
	requestService, err := zhihuRequest.NewRequestService(nil, logger)
	if err != nil {
		t.Fatal(err)
	}

	bytes, err := os.ReadFile(filepath.Join("examples", "answer_list.json"))
	if err != nil {
		t.Fatal(err)
	}

	parser, err := parse.NewParseService(parse.WithLogger(logger))
	if err != nil {
		t.Fatal(err)
	}
	paging, answerList, err := parser.ParseAnswerList(bytes, 0)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("AnswerList length: ", len(answerList))

	for _, a := range answerList {
		fmt.Println("Answer Title: ", a.Question.Title)
	}

	bytes, err = requestService.LimitRaw(paging.Next)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join("examples", "answer_list_2.json")
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

	paging, answerList, err = parser.ParseAnswerList(bytes, 1)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("AnswerList length: ", len(answerList))

	for _, a := range answerList {
		fmt.Println("Answer Title: ", a.Question.Title)
	}
}
