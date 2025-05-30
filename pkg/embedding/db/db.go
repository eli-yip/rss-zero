package db

import (
	"errors"
	"fmt"

	"github.com/pgvector/pgvector-go"
	"github.com/rs/xid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DBIface interface {
	CreateEmbedding(contentType int, contentID string, embedding []float32) (*ContentEmbedding, error)
	GetEmbedding(id string) (*ContentEmbedding, error)
	GetEmbeddingByContent(contentType int, contentID string) (*ContentEmbedding, error)
	SearchEmbedding(embedding []float32, page int, pageSize int) ([]ContentEmbedding, error)
	SearchEmbeddingByID(id string, page int, pageSize int) ([]ContentEmbedding, error)
	SearchEmbeddingByContent(contentType int, contentID string, page int, pageSize int) ([]ContentEmbedding, error)
	UpdateEmbedding(id string, embedding []float32) error
	DeleteEmbedding(id string) error

	FetchIDs() ([]string, error)
}

var ErrNotFound = errors.New("record not found")

type DBService struct{ *gorm.DB }

func NewDBService(db *gorm.DB) DBIface { return &DBService{db} }

// CreateEmbedding 创建一个新的嵌入向量记录
func (d *DBService) CreateEmbedding(contentType int, contentID string, embedding []float32) (*ContentEmbedding, error) {
	ce := &ContentEmbedding{
		ID:          xid.New().String(),
		ContentType: contentType,
		ContentID:   contentID,
		Embedding:   pgvector.NewVector(embedding),
	}

	err := d.Create(ce).Error
	if err != nil {
		return nil, err
	}

	return ce, nil
}

// GetEmbedding 通过 ID 获取嵌入向量
func (d *DBService) GetEmbedding(id string) (*ContentEmbedding, error) {
	var ce ContentEmbedding
	err := d.First(&ce, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &ce, nil
}

// GetEmbeddingByContent 通过内容类型和内容 ID 获取嵌入向量
func (d *DBService) GetEmbeddingByContent(contentType int, contentID string) (*ContentEmbedding, error) {
	var ce ContentEmbedding
	err := d.First(&ce, "content_type = ? AND content_id = ?", contentType, contentID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return &ce, nil
}

// SearchEmbedding 通过向量搜索相似嵌入，使用余弦相似度
func (d *DBService) SearchEmbedding(embedding []float32, page int, pageSize int) ([]ContentEmbedding, error) {
	var results []ContentEmbedding
	offset := (page - 1) * pageSize

	err := d.Clauses(clause.OrderBy{
		// 距离从小到大排序，代表相似从高到低
		Expression: clause.Expr{
			SQL:  "embedding <=> ?", // 使用 <=> 操作符表示余弦距离
			Vars: []any{pgvector.NewVector(embedding)},
		},
	}).Limit(pageSize).Offset(offset).Find(&results).Error

	return results, err
}

// SearchEmbeddingByID 通过 ID 搜索相似嵌入，使用余弦相似度
func (d *DBService) SearchEmbeddingByID(id string, page int, pageSize int) ([]ContentEmbedding, error) {
	// 先获取指定 ID 的嵌入向量
	ce, err := d.GetEmbedding(id)
	if err != nil {
		return nil, err
	}

	// 使用找到的嵌入向量搜索相似向量
	var results []ContentEmbedding
	offset := (page - 1) * pageSize

	err = d.Clauses(clause.OrderBy{
		Expression: clause.Expr{
			SQL:  "embedding <=> ?", // 使用 <=> 操作符表示余弦距离
			Vars: []any{ce.Embedding},
		},
	}).Where("id != ?", id).Limit(pageSize).Offset(offset).Find(&results).Error

	return results, err
}

// SearchEmbeddingByContent 通过内容类型和内容 ID 搜索相似嵌入，使用余弦相似度
func (d *DBService) SearchEmbeddingByContent(contentType int, contentID string, page int, pageSize int) ([]ContentEmbedding, error) {
	// 先获取指定内容的嵌入向量
	ce, err := d.GetEmbeddingByContent(contentType, contentID)
	if err != nil {
		return nil, err
	}

	// 使用找到的嵌入向量搜索相似向量
	var results []ContentEmbedding
	offset := (page - 1) * pageSize

	err = d.Clauses(clause.OrderBy{
		Expression: clause.Expr{
			SQL:  "embedding <=> ?", // 使用 <=> 操作符表示余弦距离
			Vars: []any{ce.Embedding},
		},
	}).Where("id != ?", ce.ID).Limit(pageSize).Offset(offset).Find(&results).Error

	return results, err
}

// UpdateEmbedding 更新嵌入向量
func (d *DBService) UpdateEmbedding(id string, embedding []float32) error {
	return d.Model(&ContentEmbedding{}).
		Where("id = ?", id).
		Update("embedding", pgvector.NewVector(embedding)).
		Error
}

// DeleteEmbedding 删除嵌入向量
func (d *DBService) DeleteEmbedding(id string) error {
	return d.Delete(&ContentEmbedding{}, "id = ?", id).Error
}

func (d *DBService) FetchIDs() (ids []string, err error) {
	if err := d.Model(&ContentEmbedding{}).Pluck("content_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("failed to get ids: %w", err)
	}
	return ids, nil
}
