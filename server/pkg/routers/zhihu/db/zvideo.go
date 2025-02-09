package db

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type DBZvideo interface {
	SaveZvideoInfo(zvideo *Zvideo) (err error)
	GetLatestZvideo() (zvideo *Zvideo, err error)
}

// A Zvideo represents a video row in the zhihu_zvideos table.
//
// We can't save video url directly because the url will expire after a while,
// but we can use id to get the video url from the zhihu api again.
type Zvideo struct {
	ID          string    `gorm:"column:id;type:text;primaryKey"`
	Filename    string    `gorm:"column:filename;type:text"`
	PublishedAt time.Time `gorm:"column:published_at;type:timestamptz"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt
	Raw         []byte `gorm:"column:raw;type:bytea"`
}

func (*Zvideo) TableName() string { return "zhihu_zvideos" }

func (d DBService) SaveZvideoInfo(zvideo *Zvideo) (err error) { return d.Create(zvideo).Error }

func (d DBService) GetLatestZvideo() (zvideo *Zvideo, err error) {
	zvideo = &Zvideo{}
	err = d.Order("published_at DESC").First(zvideo).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return zvideo, err
}
