package request

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/log"
)

func TestLimitRaw(t *testing.T) {
	logger := log.NewLogger()
	reqService, err := NewRequestService(logger)
	if err != nil {
		t.Fatal(err)
	}
	config.InitConfigFromEnv()

	params := `data[*].is_normal,admin_closed_comment,reward_info,is_collapsed,annotation_action,annotation_detail,collapse_reason,collapsed_by,suggest_edit,comment_count,can_comment,content,voteup_count,reshipment_settings,comment_permission,mark_infos,created_time,updated_time,review_info,question,excerpt,is_labeled,label_info,relationship.is_authorized,voting,is_author,is_thanked,is_nothelp;data[*].author.badge[?(type=best_answerer)].topics`
	escaped := url.QueryEscape(params)
	answerListURL := fmt.Sprintf("https://www.zhihu.com/api/v4/members/canglimo/answers?include=%s&offset=0&limit=20&sort_by=created", escaped)

	params = `data[*].is_normal,admin_closed_comment,reward_info,is_collapsed,annotation_action,annotation_detail,collapse_reason,is_sticky,collapsed_by,suggest_edit,comment_count,can_comment,content,editable_content,voteup_count,reshipment_settings,comment_permission,mark_infos,created_time,updated_time,review_info,question.detail,answer_count,follower_count,excerpt,detail,question_type,title,id,created,updated_time,relevant_info,excerpt,label_info,relationship.is_authorized,is_author,voting,is_thanked,is_nothelp,is_labeled,is_recognized`
	escaped = url.QueryEscape(params)
	singleAnswerURL := fmt.Sprintf("https://api.zhihu.com/answers/3375497152?include=%s", escaped)

	urls := []string{
		answerListURL,
		"https://www.zhihu.com/api/v4/members/canglimo/articles?offset=0&limit=20",
		singleAnswerURL,
		"https://www.zhihu.com/api/v4/articles/680621026",
		"https://www.zhihu.com/api/v4/pins/1736160848526012416",
	}

	for _, u := range urls {
		if _, err := reqService.LimitRaw(u); err != nil {
			t.Error(err)
		}
	}
}
