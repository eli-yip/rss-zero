package db

import (
	"time"

	"github.com/eli-yip/rss-zero/pkg/common"
	"gorm.io/gorm"
)

type Bookmark struct {
	ID          string                  `gorm:"primaryKey"`
	UserID      string                  `gorm:"type:text;index"`
	ContentType common.ZhihuContentType `gorm:"type:int;index"`
	ContentID   string                  `gorm:"type:text;index"`
	Comment     string                  `gorm:"type:text"`
	Note        string                  `gorm:"type:text"`

	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type Tag struct {
	BookmarkID string `gorm:"type:text;index;primaryKey"`
	Name       string `gorm:"type:text;primaryKey"`

	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
