package parse

import (
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse/models"
)

func (s *ParseService) parseAuthor(user *models.User) (author string, err error) {
	// TODO: Save user to database
	switch user.Alias {
	case nil:
		return user.Name, nil
	default:
		return *user.Alias, nil
	}
}
