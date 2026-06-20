package parse

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/eli-yip/rss-zero/internal/md"
)

const (
	answerTypePaidColumnContent  = "paid_column_content"
	articleTypePaidColumnContent = "paid_column_content"
)

// IsPaidAnswer reports whether a zhihu answer is paid column content.
func IsPaidAnswer(answerType string) bool {
	return answerType == answerTypePaidColumnContent
}

// IsPaidArticle reports whether a zhihu article is paid.
//
// NOTE: 付费判定取双条件之并。article_type=="paid_column_content" 是主信号；但实测全表
// 有 21 篇 paid_info 非空却被知乎标成 article_type=="normal" 的旧付费文，故再加 paid_info
// 非空兜底，避免漏判。详见 docs/specs/2026-06-20-01-zhihu-paid-content-flag.md。
func IsPaidArticle(articleType string, paidInfo json.RawMessage) bool {
	return articleType == articleTypePaidColumnContent || paidInfoNonEmpty(paidInfo)
}

// paidInfoNonEmpty reports whether the raw paid_info carries real content, i.e.
// is not absent, an empty object, or null.
func paidInfoNonEmpty(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s != "" && s != "{}" && s != "null"
}

// AddPaidNotice prepends a paid-content blockquote linking back to the original
// to text.
//
// Callers pass freshly rendered text (parse / refmt, both starting from the raw
// payload), which never already carries a notice, so this is a plain prepend.
// The one-off 20260620 migration handles the legacy inline notice and
// idempotency for already-stored rows, keeping this steady-state path simple.
func AddPaidNotice(text, link string) string {
	notice := md.Quote(fmt.Sprintf("本文为付费内容，请点击 [原文链接](%s) 查看全文", link))
	text = strings.TrimLeft(text, " \n")
	if text == "" {
		return notice
	}
	return notice + "\n\n" + text
}
