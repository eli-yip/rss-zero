package parse

import (
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
)

// buildAuthor 从 API user 组装作者事实行，不落库——由 SaveTopicTx 在事务内统一写。
func buildAuthor(user *models.User) *db.Author {
	return &db.Author{
		ID:    user.UserID,
		Name:  user.Name,
		Alias: user.Alias,
	}
}

// displayName 返回作者展示名（有别名取别名），仅用于屏蔽名单匹配等抓取期判断；
// 读取期正文/标题渲染统一取 db.Author.Name（见 render.RenderMarkdown）。
func displayName(a *db.Author) string {
	if a.Alias != nil {
		return *a.Alias
	}
	return a.Name
}
