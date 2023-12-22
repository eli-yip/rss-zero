package parse

import (
	"github.com/eli-yip/zsxq-parser/pkg/parse/models"
)

func (s *ParseService) parseAuthor(user *models.User) (author string, err error) {
	switch user.Alias {
	case nil:
		return user.Name, nil
	default:
		return *user.Alias, nil
	}
}
