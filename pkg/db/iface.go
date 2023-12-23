package db

import "github.com/eli-yip/zsxq-parser/pkg/db/models"

type DataBaseIface interface {
	SaveTopic(*models.Topic) error
	SaveObject(*models.Object) error
	GetObjectInfo(id int) (string, []string, error)
}
