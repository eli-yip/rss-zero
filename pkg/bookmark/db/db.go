package db

import (
	"errors"
	"fmt"
	"slices"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/rs/xid"
	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNoBookmark = errors.New("no bookmark found")
)

type BookmarkQueryTime int

const (
	BookmarkQueryCreateTime BookmarkQueryTime = iota
	BookmarkQueryUpdateTime
)

type BookmarkQuery struct {
	StartTime time.Time
	EndTime   time.Time
	TimeBy    BookmarkQueryTime
	Page      int
	Order     int
	Orderby   BookmarkQueryTime
	Tag       *TagFilter
}

type TagFilter struct {
	Include []string
	Exclude []string
	NoTag   bool
}

type TagCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type DB interface {
	NewBookmark(userID string, contentType int, contentID string) (*Bookmark, error)
	GetBookmark(userID, bookmarkID string) (*Bookmark, error)
	GetBookmarkByUser(userID string, q *BookmarkQuery) ([]Bookmark, error)
	CountBookmarkByUser(userID string, q *BookmarkQuery) (int, error)
	// GetBookmarkByContent returns ErrNoBookmark if no bookmark found
	GetBookmarkByContent(userID string, contentType int, contentID string) (*Bookmark, error)
	GetBookmarkByTag(userID, tagName string) ([]Bookmark, error)
	GetBookmarkByTags(userID string, tagNames []string) ([]Bookmark, error)
	UpdateBookmark(id string, comment, note string) (*Bookmark, error)
	RemoveBookmark(id string) error
	UpdateBookmarkUpdatedAt(id string) error

	UpdateTag(bookmarkID string, newTags []string) error
	AddTag(bookmarkID string, name string) (*Tag, error)
	AddTags(bookmarkID string, tagNames []string) ([]Tag, error)
	RemoveTag(bookmarkID string, name string) error
	RemoveTags(bookmarkID string, tagNames []string) error
	// The result is ordered by created_at ASC.
	// If a  bookmark has no tag, it will return an empty slice
	GetTag(bookmarkID string) ([]string, error)
	// The result is ordered by created_at ASC.
	// If a bookmark has no tags, the value will be nil
	GetTags(bookmarkIDs []string) (map[string][]string, error)
	// GetTagByUser returns all tag names for a user
	GetTagByUser(userID string) ([]string, error)
	// GetTagCountByUser returns the count of each tag for a user, ordered by count
	GetTagCountByUser(userID string) ([]TagCount, error)
}

type BookmarkDBImpl struct{ *gorm.DB }

func NewBookMarkDBImpl(db *gorm.DB) DB { return &BookmarkDBImpl{db} }

func (db *BookmarkDBImpl) NewBookmark(userID string, contentType int, contentID string) (*Bookmark, error) {
	bookmark := &Bookmark{
		ID:          xid.New().String(),
		UserID:      userID,
		ContentType: contentType,
		ContentID:   contentID,
	}

	if err := db.Create(&bookmark).Error; err != nil {
		return nil, err
	}

	return bookmark, nil
}

func (db *BookmarkDBImpl) GetBookmark(userID, bookmarkID string) (*Bookmark, error) {
	bookmark := &Bookmark{}
	result := db.Where("user_id = ? AND id = ?", userID, bookmarkID).First(&bookmark)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrNoBookmark
		}
		return nil, result.Error
	}
	return bookmark, nil
}

const pageSize = 20

func (db *BookmarkDBImpl) GetBookmarkByUser(userID string, q *BookmarkQuery) ([]Bookmark, error) {
	bookmarks := make([]Bookmark, 0)
	stmt := db.Model(&Bookmark{}).Where("user_id = ?", userID)
	if q != nil {
		var timeFilter string
		switch q.TimeBy {
		case BookmarkQueryCreateTime:
			timeFilter = "created_at"
		case BookmarkQueryUpdateTime:
			timeFilter = "updated_at"
		}
		if !q.StartTime.IsZero() {
			stmt = stmt.Where(timeFilter+" >= ?", q.StartTime)
		}
		if !q.EndTime.IsZero() {
			stmt = stmt.Where(timeFilter+" <= ?", q.EndTime)
		}
		var orderBy string
		switch q.Orderby {
		case BookmarkQueryCreateTime:
			orderBy = "created_at"
		case BookmarkQueryUpdateTime:
			orderBy = "updated_at"
		}
		if q.Order == 0 {
			stmt = stmt.Order(orderBy + " DESC")
		} else {
			stmt = stmt.Order(orderBy + " ASC")
		}
		if q.Page == 0 {
			q.Page = 1
		}
		offset := (q.Page - 1) * pageSize
		stmt = stmt.Offset(offset).Limit(pageSize)

		if q.Tag != nil {
			if q.Tag.NoTag {
				var bookmarkIDs []string
				db.Model(&Tag{}).Distinct().Pluck("bookmark_id", &bookmarkIDs)
				if len(bookmarkIDs) > 0 {
					stmt = stmt.Where("id NOT IN ?", bookmarkIDs)
				}
			} else {
				if len(q.Tag.Include) > 0 {
					var bookmarkIDs []string
					db.Model(&Tag{}).Where("name IN ?", q.Tag.Include).Distinct().Pluck("bookmark_id", &bookmarkIDs)
					if len(bookmarkIDs) > 0 {
						stmt = stmt.Where("id IN ?", bookmarkIDs)
					} else {
						stmt = stmt.Where("1 = 0") // no bookmark found
					}
				}

				if len(q.Tag.Exclude) > 0 {
					var bookmarkIDs []string
					db.Model(&Tag{}).Where("name IN ?", q.Tag.Exclude).Distinct().Pluck("bookmark_id", &bookmarkIDs)
					if len(bookmarkIDs) > 0 {
						stmt = stmt.Where("id NOT IN ?", bookmarkIDs)
					}
				}
			}
		}
	}
	result := stmt.Find(&bookmarks)
	if result.Error != nil {
		return nil, result.Error
	}
	return bookmarks, nil
}

func (db *BookmarkDBImpl) CountBookmarkByUser(userID string, q *BookmarkQuery) (int, error) {
	var count int64
	stmt := db.Model(&Bookmark{}).Where("user_id = ?", userID)
	if q != nil {
		var timeFilter string
		switch q.TimeBy {
		case BookmarkQueryCreateTime:
			timeFilter = "created_at"
		case BookmarkQueryUpdateTime:
			timeFilter = "updated_at"
		}
		if !q.StartTime.IsZero() {
			stmt = stmt.Where(timeFilter+" >= ?", q.StartTime)
		}
		if !q.EndTime.IsZero() {
			stmt = stmt.Where(timeFilter+" <= ?", q.EndTime)
		}

		if q.Tag != nil {
			if q.Tag.NoTag {
				var bookmarkIDs []string
				db.Model(&Tag{}).Distinct().Pluck("bookmark_id", &bookmarkIDs)
				if len(bookmarkIDs) > 0 {
					stmt = stmt.Where("id NOT IN ?", bookmarkIDs)
				}
			} else {
				if len(q.Tag.Include) > 0 {
					var bookmarkIDs []string
					db.Model(&Tag{}).Where("name IN ?", q.Tag.Include).Distinct().Pluck("bookmark_id", &bookmarkIDs)
					if len(bookmarkIDs) > 0 {
						stmt = stmt.Where("id IN ?", bookmarkIDs)
					} else {
						stmt = stmt.Where("1 = 0") // no bookmark found
					}
				}

				if len(q.Tag.Exclude) > 0 {
					var bookmarkIDs []string
					db.Model(&Tag{}).Where("name IN ?", q.Tag.Exclude).Distinct().Pluck("bookmark_id", &bookmarkIDs)
					if len(bookmarkIDs) > 0 {
						stmt = stmt.Where("id NOT IN ?", bookmarkIDs)
					}
				}
			}
		}
	}

	result := stmt.Count(&count)
	if result.Error != nil {
		return 0, result.Error
	}
	return int(count), nil
}

func (db *BookmarkDBImpl) GetBookmarkByContent(userID string, contentType int, contentID string) (*Bookmark, error) {
	bookmark := &Bookmark{}
	result := db.Where("user_id = ? AND content_type = ? AND content_id = ?", userID, contentType, contentID).First(&bookmark)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrNoBookmark
		}
		return nil, result.Error
	}
	return bookmark, nil
}

func (db *BookmarkDBImpl) UpdateBookmark(id string, comment, note string) (*Bookmark, error) {
	updatedBookmark := &Bookmark{}
	result := db.Model(updatedBookmark).Where("id = ?", id).Clauses(clause.Returning{}).Updates(map[string]any{
		"comment": comment,
		"note":    note,
	}) // use map here to update no-zero fields: https://gorm.io/docs/update.html#Updates-multiple-columns

	if result.Error != nil {
		return nil, result.Error
	}
	return updatedBookmark, nil
}

func (db *BookmarkDBImpl) RemoveBookmark(id string) error {
	result := db.Delete(&Bookmark{ID: id})
	if result.Error != nil {
		return result.Error
	}

	var tags []string
	err := db.Model(&Tag{}).Where("bookmark_id = ?", id).Pluck("name", &tags).Error
	if err != nil {
		return err
	}
	return db.RemoveTags(id, tags)
}

func (db *BookmarkDBImpl) UpdateTag(bookmarkID string, newTags []string) error {
	if len(newTags) == 0 {
		return nil
	}

	// Remove old tags
	var oldTags []string
	if err := db.Model(&Tag{}).Where("bookmark_id = ?", bookmarkID).Distinct().Pluck("name", &oldTags).Error; err != nil {
		return fmt.Errorf("failed to get old tags: %w", err)
	}

	newTagSet := mapset.NewSet(newTags...)
	oldTagSet := mapset.NewSet(oldTags...)
	tagsNeedToAdd := newTagSet.Difference(newTagSet.Intersect(oldTagSet))
	tagsNeedToRemove := oldTagSet.Difference(newTagSet.Intersect(oldTagSet))

	if err := db.RemoveTags(bookmarkID, tagsNeedToRemove.ToSlice()); err != nil {
		return fmt.Errorf("failed to remove old tags: %w", err)
	}
	if _, err := db.AddTags(bookmarkID, tagsNeedToAdd.ToSlice()); err != nil {
		return fmt.Errorf("failed to add new tags: %w", err)
	}

	// Update the updated_at field of the bookmark
	if err := db.Model(&Bookmark{}).Where("id = ?", bookmarkID).Update("updated_at", time.Now()).Error; err != nil {
		return fmt.Errorf("failed to update bookmark updated_at: %w", err)
	}

	return nil
}

func (db *BookmarkDBImpl) UpdateBookmarkUpdatedAt(id string) error {
	result := db.Model(&Bookmark{}).Where("id = ?", id).Update("updated_at", time.Now())
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (db *BookmarkDBImpl) AddTag(bookmarkID string, name string) (*Tag, error) {
	tag := &Tag{
		BookmarkID: bookmarkID,
		Name:       name,
	}
	// Use Save here because we use bookmark_id and name as primary key,
	// which may cause duplicate key error during creating after soft delete
	if err := db.Save(&tag).Error; err != nil {
		return nil, err
	}
	return tag, nil
}

func (db *BookmarkDBImpl) AddTags(bookmarkID string, tagNames []string) ([]Tag, error) {
	if len(tagNames) == 0 {
		return nil, nil
	}
	tags := lo.Map(tagNames, func(name string, _ int) Tag {
		return Tag{
			BookmarkID: bookmarkID,
			Name:       name,
		}
	})
	if err := db.Save(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

func (db *BookmarkDBImpl) RemoveTag(bookmarkID string, name string) error {
	result := db.Where("bookmark_id = ? AND name = ?", bookmarkID, name).Delete(&Tag{})
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (db *BookmarkDBImpl) RemoveTags(bookmarkID string, tagNames []string) error {
	if len(tagNames) == 0 {
		return nil
	}
	if err := db.Where("bookmark_id = ? AND name in ?", bookmarkID, tagNames).Delete(&Tag{}).Error; err != nil {
		return err
	}
	return nil
}

func (db *BookmarkDBImpl) GetTag(bookmarkID string) ([]string, error) {
	tags := make([]Tag, 0)
	result := db.Where("bookmark_id = ?", bookmarkID).Order("created_at ASC").Find(&tags)
	if result.Error != nil {
		return nil, result.Error
	}
	tagNames := lo.Map(tags, func(t Tag, _ int) string {
		return t.Name
	})
	return tagNames, nil
}

func (db *BookmarkDBImpl) GetTags(bookmarkIDs []string) (map[string][]string, error) {
	tags := make([]Tag, 0)
	result := db.Where("bookmark_id IN ?", bookmarkIDs).Order("created_at ASC").Find(&tags)
	if result.Error != nil {
		return nil, result.Error
	}
	if len(tags) == 0 {
		return nil, nil
	}
	tagMap := make(map[string][]string)
	for t := range slices.Values(tags) {
		tagMap[t.BookmarkID] = append(tagMap[t.BookmarkID], t.Name)
	}
	return tagMap, nil
}

func (db *BookmarkDBImpl) GetTagByUser(userID string) ([]string, error) {
	var tags []string

	var bookmarkIDs []string
	if err := db.Model(&Bookmark{}).
		Where("user_id = ?", userID).
		Pluck("id", &bookmarkIDs).Error; err != nil {
		return nil, err
	}

	if len(bookmarkIDs) == 0 {
		return tags, nil
	}

	if err := db.Model(&Tag{}).
		Where("bookmark_id IN ?", bookmarkIDs).
		Distinct().
		Pluck("name", &tags).Error; err != nil {
		return nil, err
	}

	return tags, nil
}

func (db *BookmarkDBImpl) GetBookmarkByTag(userID, tagName string) ([]Bookmark, error) {
	bookmarks := make([]Bookmark, 0)

	err := db.Model(&Bookmark{}).Joins("JOIN tags ON tags.bookmark_id = bookmarks.id").
		Where("tags.name = ? AND tags.deleted_at IS NULL AND bookmarks.user_id = ?", tagName, userID).
		Find(&bookmarks).Error

	if err != nil {
		return nil, err
	}

	return bookmarks, nil
}

func (db *BookmarkDBImpl) GetBookmarkByTags(userID string, tagNames []string) ([]Bookmark, error) {
	var bookmarks []Bookmark

	if len(tagNames) == 0 {
		return bookmarks, nil
	}

	err := db.Model(&Bookmark{}).Distinct().
		Joins("JOIN tags ON tags.bookmark_id = bookmarks.id").
		Where("tags.name IN ? AND tags.deleted_at IS NULL AND bookmarks.user_id = ?", tagNames, userID).
		Find(&bookmarks).Error

	if err != nil {
		return nil, err
	}

	return bookmarks, nil
}

func (d *BookmarkDBImpl) GetTagCountByUser(userID string) ([]TagCount, error) {
	var tagCounts []TagCount

	err := d.Model(&Tag{}).
		Select("tags.name, COUNT(tags.name) as count").
		Joins("JOIN bookmarks ON bookmarks.id = tags.bookmark_id").
		Where("bookmarks.user_id = ?", userID).
		Group("name").
		Order("count DESC").
		Find(&tagCounts).Error

	if err != nil {
		return nil, err
	}

	return tagCounts, nil
}
