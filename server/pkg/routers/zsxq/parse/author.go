package parse

import (
	"fmt"

	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
)

func (s *ParseService) parseAuthor(user *models.User) (id int, name string, err error) {
	err = s.db.SaveAuthor(&db.Author{
		ID:    user.UserID,
		Name:  user.Name,
		Alias: user.Alias,
	})
	if err != nil {
		return 0, "", fmt.Errorf("failed to save author info: %w", err)
	}

	switch user.Alias {
	case nil:
		return user.UserID, user.Name, nil
	default:
		return user.UserID, *user.Alias, nil
	}
}
