package db

import (
	"time"

	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
)

type ContentEmbedding struct {
	ID          string          `gorm:"primaryKey"`
	ContentType int             `gorm:"type:int;index"`
	ContentID   string          `gorm:"type:text;index"`
	Embedding   pgvector.Vector `gorm:"type:vector(2048)"`

	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (e *ContentEmbedding) TableName() string { return "content_embedding" }
