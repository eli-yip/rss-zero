package render

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"

	"github.com/eli-yip/rss-zero/internal/md"
)

// 本文件是读取期纯渲染所需的小纯函数。付费判定与付费提示（isPaidAnswer/isPaidArticle/
// paidInfoNonEmpty/addPaidNotice）以及图片换链键（URLToID/FindImageLinks）现已是唯一实现——
// parse 的写侧副本随 refmt 一并删除（P2.4）。抓取期 parse.downloadImageObjects / parsePinContent
// 直接复用这里导出的 URLToID/FindImageLinks，保证读写换链键逐字节一致。

// URLToID 把 url 哈希成一个 int id（object.ID / 图片换链的键）。
func URLToID(str string) int {
	h := fnv.New32a()
	h.Write([]byte(str))
	return int(h.Sum32())
}

var imageLinkRe = regexp.MustCompile(`!\[.*?\]\((.*?)\)`)

// FindImageLinks 取出 markdown 里所有图片链接的 url。
func FindImageLinks(text string) (links []string) {
	matches := imageLinkRe.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) > 1 {
			links = append(links, match[1])
		}
	}
	return links
}

// replaceImageLink 把正文里指向 from 的图片语法整体换成 ![name](to)。
func replaceImageLink(text, name, from, to string) string {
	re := regexp.MustCompile(`!\[[^\]]*\]\(` + regexp.QuoteMeta(from) + `\)`)
	return re.ReplaceAllString(text, `![`+name+`](`+to+`)`)
}

const (
	answerTypePaidColumnContent  = "paid_column_content"
	articleTypePaidColumnContent = "paid_column_content"
)

// isPaidAnswer 判断 answer 是否付费专栏内容。
func isPaidAnswer(answerType string) bool { return answerType == answerTypePaidColumnContent }

// isPaidArticle 判断 article 是否付费：article_type 为付费专栏，或 paid_info 非空兜底。
func isPaidArticle(articleType string, paidInfo json.RawMessage) bool {
	return articleType == articleTypePaidColumnContent || paidInfoNonEmpty(paidInfo)
}

// paidInfoNonEmpty 判断 raw paid_info 是否携带真实内容（非缺失/空对象/null）。
func paidInfoNonEmpty(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s != "" && s != "{}" && s != "null"
}

// addPaidNotice 在正文前置一段指向原文的付费提示引用块。
func addPaidNotice(text, link string) string {
	notice := md.Quote(fmt.Sprintf("本文为付费内容，请点击 [原文链接](%s) 查看全文", link))
	text = strings.TrimLeft(text, " \n")
	if text == "" {
		return notice
	}
	return notice + "\n\n" + text
}

// TryToFindTitle 在 \| 处切出标题：命中则前段为标题、后段为正文；未命中则标题为空、正文不变。
// 读侧（renderPinContent）与写侧（parse.parsePinContent）共用这一份，保证标题切分逐字节一致。
func TryToFindTitle(text string) (title, content string) {
	title, content, found := strings.Cut(text, `\|`)
	if found {
		return title, content
	}
	return "", text
}

// formatMarkdown 每次新建 formatter 跑一遍 markdown 格式化，保证无共享状态、确定性。
func formatMarkdown(text string) (string, error) {
	return md.NewMarkdownFormatter().FormatStr(text)
}
