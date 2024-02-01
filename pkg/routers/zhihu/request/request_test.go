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
	reqService := NewRequestService(logger)
	config.InitConfigFromEnv()

	params := `data[*].is_normal,admin_closed_comment,reward_info,is_collapsed,annotation_action,annotation_detail,collapse_reason,is_sticky,collapsed_by,suggest_edit,comment_count,can_comment,content,editable_content,voteup_count,reshipment_settings,comment_permission,mark_infos,created_time,updated_time,review_info,question.detail,answer_count,follower_count,excerpt,detail,question_type,title,id,created,updated_time,relevant_info,excerpt,label_info,relationship.is_authorized,is_author,voting,is_thanked,is_nothelp,is_labeled,is_recognized`
	escaped := url.QueryEscape(params)

	urls := []string{
		fmt.Sprintf("https://api.zhihu.com/answers/3375497152?include=%s", escaped),
		"https://www.zhihu.com/api/v4/articles/680621026",
		"https://www.zhihu.com/api/v4/pins/1736160848526012416",
	}

	for _, u := range urls {
		_, err := reqService.LimitRaw(u)
		if err != nil {
			t.Error(err)
		}
	}
}
