package migrate

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

type legacyTombkeeperPost struct {
	ID           int64  `gorm:"column:id"`
	TextMarkdown string `gorm:"column:text_markdown"`
	Raw          []byte `gorm:"column:raw"`
}

func (*legacyTombkeeperPost) TableName() string { return "tombkeeper_post" }

var (
	legacyRetweetHead     = regexp.MustCompile(`(?m)^> 转发 @`)
	legacyInlineQuoteHead = regexp.MustCompile(`^> 微博正文\d+ @`)
	legacyBeijing         = time.FixedZone("Beijing", 8*3600)
)

func reorderLegacyInlineQuotes(body string) string {
	loc := legacyRetweetHead.FindStringIndex(body)
	if loc == nil {
		return body
	}
	head, tail := body[:loc[0]], body[loc[0]:]
	blocks := strings.Split(tail, "\n\n")
	retweet, rest := blocks[0], blocks[1:]
	var inline, others []string
	for _, block := range rest {
		if legacyInlineQuoteHead.MatchString(block) {
			inline = append(inline, block)
		} else {
			others = append(others, block)
		}
	}
	if len(inline) == 0 {
		return body
	}
	return head + strings.Join(append(append(inline, retweet), others...), "\n\n")
}

func appendLegacyRetweetTime(body string, raw []byte) string {
	loc := legacyRetweetHead.FindStringIndex(body)
	if loc == nil {
		return body
	}
	var payload struct {
		RetweetWeibo struct {
			CreatedAt string `json:"created_at"`
		} `json:"retweet_weibo"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return body
	}
	createdAt, err := time.Parse(time.RFC3339, strings.TrimPrefix(payload.RetweetWeibo.CreatedAt, "$D"))
	if err != nil {
		return body
	}
	line := createdAt.In(legacyBeijing).Format("2006 年 01 月 02 日 15:04")
	head, tail := body[:loc[0]], body[loc[0]:]
	blocks := strings.Split(tail, "\n\n")
	if strings.HasSuffix(blocks[0], "> "+line) {
		return body
	}
	blocks[0] += "\n> \n> " + line
	return head + strings.Join(blocks, "\n\n")
}
