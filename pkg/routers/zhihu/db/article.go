package db

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type Article struct {
	ID       int       `gorm:"column:id;type:int;primary_key"`
	AuthorID string    `gorm:"column:author_id;type:text"`
	CreateAt time.Time `gorm:"column:create_at;type:timestamptz"`
	UpdateAt time.Time `gorm:"column:update_at;type:timestamptz"`
	Title    string    `gorm:"column:title;type:text"`
	Raw      []byte    `gorm:"column:raw;type:bytea"`
}

func (p *Article) TableName() string { return "zhihu_article" }

type DBArticle interface {
	GetArticle(id int) (*Article, error)
	SaveArticle(p *Article) error
	// SaveArticleTx 在单事务内提交一条 article 产生的全部行：作者、图片对象、article 根行。
	// 原子性来自事务本身（任一步失败整体回滚，见 plan 决策 4）；根行最后写只是可读性约定，
	// 无 FK 强制、不改变回滚语义。
	SaveArticleTx(article *Article, author *Author, objects []Object) error
	GetLatestNArticle(n int, authorID string) ([]Article, error)
	GetLatestArticleTime(authorID string) (time.Time, error)
	FetchNArticle(n int, opt FetchArticleOption) (as []Article, err error)
	GetArticleAfter(authorID string, t time.Time) ([]Article, error)
	CountArticle(authorID string) (int, error)
	FetchNArticlesBeforeTime(n int, t time.Time, authorID string) (as []Article, err error)
}

func (d *DBService) GetArticle(id int) (*Article, error) {
	var a Article
	if err := d.First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (d *DBService) SaveArticle(p *Article) error { return d.Save(p).Error }

// SaveArticleTx 把一条 article 解析出的全部事实行放进同一个事务提交：作者、各图片对象、
// article 根行。原子性来自事务本身（任一步失败整体回滚）；根行最后写只是可读性约定，无 FK
// 强制、不改变回滚语义。图片 OSS 上传在事务外完成，见 SaveAnswerTx 注释。
func (d *DBService) SaveArticleTx(article *Article, author *Author, objects []Object) error {
	return d.Transaction(func(tx *gorm.DB) error {
		if author != nil {
			if err := tx.Save(author).Error; err != nil {
				return fmt.Errorf("failed to save author %s: %w", author.ID, err)
			}
		}
		for i := range objects {
			if err := tx.Save(&objects[i]).Error; err != nil {
				return fmt.Errorf("failed to save object %d: %w", objects[i].ID, err)
			}
		}
		if err := tx.Save(article).Error; err != nil {
			return fmt.Errorf("failed to save article root %d: %w", article.ID, err)
		}
		return nil
	})
}

func (d *DBService) GetLatestNArticle(n int, authorID string) ([]Article, error) {
	as := make([]Article, 0, n)
	if err := d.Where("author_id = ?", authorID).Order("create_at desc").Limit(n).Find(&as).Error; err != nil {
		return nil, err
	}
	return as, nil
}

func (d *DBService) GetLatestArticleTime(userID string) (time.Time, error) {
	var t time.Time
	if err := d.Model(&Article{}).Where("author_id = ?", userID).Order("create_at desc").Limit(1).Pluck("create_at", &t).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return t, nil
}

func (d *DBService) FetchNArticlesBeforeTime(n int, t time.Time, authorID string) (as []Article, err error) {
	err = d.Where("author_id = ? and create_at < ?", authorID, t).Order("create_at desc").Limit(n).Find(&as).Error
	return as, err
}

func (d *DBService) CountArticle(authorID string) (int, error) {
	var count int64
	if err := d.Model(&Article{}).Where("author_id = ?", authorID).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

type FetchArticleOption struct{ FetchOptionBase }

func (d *DBService) FetchNArticle(n int, opt FetchArticleOption) (as []Article, err error) {
	as = make([]Article, 0, n)

	query := d.Limit(n)

	if opt.UserID != nil {
		query = query.Where("author_id = ?", *opt.UserID)
	}

	if !opt.StartTime.IsZero() {
		query = query.Where("create_at >= ?", opt.StartTime)
	}

	if !opt.EndTime.IsZero() {
		query = query.Where("create_at <= ?", opt.EndTime)
	}

	if err := query.Order("create_at asc").Find(&as).Error; err != nil {
		return nil, err
	}

	return as, nil
}

func (d *DBService) GetArticleAfter(authorID string, t time.Time) ([]Article, error) {
	var as []Article
	if err := d.Where("author_id = ? and create_at > ?", authorID, t).Order("create_at asc").Find(&as).Error; err != nil {
		return nil, err
	}
	return as, nil
}
