package parse

import (
	dbModels "github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db/models"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse/models"
)

func (s *ParseService) parseAuthor(u *models.User) (id int, name string, err error) {
	go func(u *models.User) {
		err = s.DB.SaveAuthorInfo(&dbModels.Author{
			ID:    u.UserID,
			Name:  u.Name,
			Alias: u.Alias,
		})
		if err != nil {
			// TODO: log error
			return
		}
	}(u)

	switch u.Alias {
	case nil:
		return u.UserID, u.Name, nil
	default:
		return u.UserID, *u.Alias, nil
	}
}
