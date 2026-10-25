package weibochat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrBookmarkNotFound = errors.New("bookmark block not found")
var ErrInvalidBookmarkCursor = errors.New("invalid bookmark cursor")

type BookmarkBlock struct {
	ID            string    `gorm:"type:uuid;primaryKey"`
	OwnerUsername string    `gorm:"type:text;not null;index:idx_weibo_bookmark_owner_updated,priority:1"`
	Title         string    `gorm:"type:text;not null"`
	CreatedAt     time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt     time.Time `gorm:"type:timestamptz;not null;index:idx_weibo_bookmark_owner_updated,priority:2,sort:desc"`
	Tags          []string  `gorm:"-"`
	MessageCount  int64     `gorm:"-"`
}

func (BookmarkBlock) TableName() string { return "weibo_chat_bookmark_block" }

type BookmarkMessage struct {
	BlockID   string    `gorm:"type:uuid;primaryKey"`
	GroupID   string    `gorm:"type:text;primaryKey;index:idx_weibo_bookmark_message,priority:1"`
	MessageID string    `gorm:"type:text;primaryKey;index:idx_weibo_bookmark_message,priority:2"`
	AddedAt   time.Time `gorm:"type:timestamptz;not null"`
}

func (BookmarkMessage) TableName() string { return "weibo_chat_bookmark_message" }

type BookmarkTag struct {
	BlockID string `gorm:"type:uuid;primaryKey"`
	Name    string `gorm:"type:text;primaryKey;index"`
}

func (BookmarkTag) TableName() string { return "weibo_chat_bookmark_tag" }

type MessageRef struct {
	GroupID   string `json:"group_id"`
	MessageID string `json:"message_id"`
}

type BookmarkBlockFilter struct {
	Title   string
	Tags    []string
	Cursor  string
	Limit   int
	Message *MessageRef
}
type BookmarkBlockPage struct {
	Blocks     []BookmarkBlock
	NextCursor string
}
type BookmarkMessagePage struct {
	Messages   []Message
	NextCursor string
}
type bookmarkBlockCursor struct {
	UpdatedAt time.Time `json:"updated_at"`
	ID        string    `json:"id"`
}
type bookmarkMessageCursor struct {
	SentAt    *time.Time `json:"sent_at"`
	GroupID   string     `json:"group_id"`
	MessageID string     `json:"message_id"`
}

func encodeBookmarkCursor(value any) string {
	data, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(data)
}
func decodeBookmarkCursor(value string, target any) error {
	if len(value) > 2048 {
		return ErrInvalidBookmarkCursor
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return ErrInvalidBookmarkCursor
	}
	if json.Unmarshal(data, target) != nil {
		return ErrInvalidBookmarkCursor
	}
	return nil
}

func bookmarkOwnerQuery(db *gorm.DB, owner, id string) *gorm.DB {
	return db.Where("owner_username = ? AND id = ?", owner, id)
}
func findBookmark(db *gorm.DB, owner, id string, lock bool) (BookmarkBlock, error) {
	var block BookmarkBlock
	q := bookmarkOwnerQuery(db, owner, id)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := q.First(&block).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ErrBookmarkNotFound
	}
	return block, err
}

func (s *PostgresStore) CreateBookmarkBlock(ctx context.Context, owner, title string, tags []string, refs []MessageRef) (BookmarkBlock, error) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	block := BookmarkBlock{ID: uuid.NewString(), OwnerUsername: owner, Title: title, CreatedAt: now, UpdatedAt: now}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&block).Error; err != nil {
			return err
		}
		if err := replaceBookmarkTags(tx, block.ID, tags); err != nil {
			return err
		}
		_, err := addBookmarkMessages(tx, block.ID, refs, now)
		return err
	})
	if err != nil {
		return BookmarkBlock{}, err
	}
	return s.GetBookmarkBlock(ctx, owner, block.ID)
}
func replaceBookmarkTags(tx *gorm.DB, id string, tags []string) error {
	if err := tx.Where("block_id = ?", id).Delete(&BookmarkTag{}).Error; err != nil {
		return err
	}
	rows := make([]BookmarkTag, 0, len(tags))
	for _, tag := range tags {
		rows = append(rows, BookmarkTag{BlockID: id, Name: tag})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}
func addBookmarkMessages(tx *gorm.DB, id string, refs []MessageRef, now time.Time) (int64, error) {
	rows := make([]BookmarkMessage, 0, len(refs))
	seen := map[MessageRef]bool{}
	for _, ref := range refs {
		if seen[ref] {
			continue
		}
		seen[ref] = true
		rows = append(rows, BookmarkMessage{BlockID: id, GroupID: ref.GroupID, MessageID: ref.MessageID, AddedAt: now})
	}
	if len(rows) == 0 {
		return 0, nil
	}
	pairs := make([][]any, 0, len(rows))
	for _, r := range rows {
		pairs = append(pairs, []any{r.GroupID, r.MessageID})
	}
	var count int64
	if err := tx.Model(&Message{}).Where("(group_id, message_id) IN ?", pairs).Count(&count).Error; err != nil {
		return 0, err
	}
	if count != int64(len(rows)) {
		return 0, ErrMessageNotFound
	}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows)
	return result.RowsAffected, result.Error
}
func (s *PostgresStore) GetBookmarkBlock(ctx context.Context, owner, id string) (BookmarkBlock, error) {
	block, err := findBookmark(s.db.WithContext(ctx), owner, id, false)
	if err != nil {
		return block, err
	}
	blocks := []BookmarkBlock{block}
	err = s.populateBookmarkBlocks(ctx, blocks)
	return blocks[0], err
}
func (s *PostgresStore) populateBookmarkBlocks(ctx context.Context, blocks []BookmarkBlock) error {
	if len(blocks) == 0 {
		return nil
	}
	ids := make([]string, 0, len(blocks))
	for i := range blocks {
		blocks[i].Tags = []string{}
		ids = append(ids, blocks[i].ID)
	}
	var tags []BookmarkTag
	if err := s.db.WithContext(ctx).Where("block_id IN ?", ids).Order("name COLLATE \"C\"").Find(&tags).Error; err != nil {
		return err
	}
	var counts []struct {
		BlockID string
		Count   int64
	}
	if err := s.db.WithContext(ctx).Model(&BookmarkMessage{}).Select("block_id, count(*) AS count").Where("block_id IN ?", ids).Group("block_id").Scan(&counts).Error; err != nil {
		return err
	}
	byID := map[string]*BookmarkBlock{}
	for i := range blocks {
		byID[blocks[i].ID] = &blocks[i]
	}
	for _, tag := range tags {
		byID[tag.BlockID].Tags = append(byID[tag.BlockID].Tags, tag.Name)
	}
	for _, count := range counts {
		byID[count.BlockID].MessageCount = count.Count
	}
	return nil
}
func (s *PostgresStore) UpdateBookmarkBlock(ctx context.Context, owner, id string, title *string, tags *[]string) (BookmarkBlock, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := findBookmark(tx, owner, id, true); err != nil {
			return err
		}
		if tags != nil {
			if err := replaceBookmarkTags(tx, id, *tags); err != nil {
				return err
			}
		}
		changes := map[string]any{"updated_at": time.Now().UTC()}
		if title != nil {
			changes["title"] = *title
		}
		return bookmarkOwnerQuery(tx.Model(&BookmarkBlock{}), owner, id).Updates(changes).Error
	})
	if err != nil {
		return BookmarkBlock{}, err
	}
	return s.GetBookmarkBlock(ctx, owner, id)
}
func (s *PostgresStore) AddBookmarkMessages(ctx context.Context, owner, id string, refs []MessageRef) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := findBookmark(tx, owner, id, true); err != nil {
			return err
		}
		now := time.Now().UTC()
		n, err := addBookmarkMessages(tx, id, refs, now)
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
		return bookmarkOwnerQuery(tx.Model(&BookmarkBlock{}), owner, id).Update("updated_at", now).Error
	})
}
func (s *PostgresStore) RemoveBookmarkMessage(ctx context.Context, owner, id string, ref MessageRef) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := findBookmark(tx, owner, id, true); err != nil {
			return err
		}
		result := tx.Where("block_id = ? AND group_id = ? AND message_id = ?", id, ref.GroupID, ref.MessageID).Delete(&BookmarkMessage{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return bookmarkOwnerQuery(tx.Model(&BookmarkBlock{}), owner, id).Update("updated_at", time.Now().UTC()).Error
	})
}
func (s *PostgresStore) DeleteBookmarkBlock(ctx context.Context, owner, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := findBookmark(tx, owner, id, true); err != nil {
			return err
		}
		if err := tx.Where("block_id = ?", id).Delete(&BookmarkMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("block_id = ?", id).Delete(&BookmarkTag{}).Error; err != nil {
			return err
		}
		return bookmarkOwnerQuery(tx, owner, id).Delete(&BookmarkBlock{}).Error
	})
}
func (s *PostgresStore) ListBookmarkBlocks(ctx context.Context, owner string, filter BookmarkBlockFilter) (BookmarkBlockPage, error) {
	page := BookmarkBlockPage{Blocks: []BookmarkBlock{}}
	if filter.Limit < 1 || filter.Limit > 100 {
		return page, fmt.Errorf("limit must be between 1 and 100")
	}
	q := s.db.WithContext(ctx).Model(&BookmarkBlock{}).Where("owner_username = ?", owner)
	if filter.Title != "" {
		q = q.Where(`translate(title, 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz') COLLATE "C" LIKE ? ESCAPE '!'`, searchPattern(filter.Title))
	}
	for _, tag := range filter.Tags {
		q = q.Where("EXISTS (SELECT 1 FROM weibo_chat_bookmark_tag t WHERE t.block_id = weibo_chat_bookmark_block.id AND t.name = ?)", tag)
	}
	if filter.Message != nil {
		q = q.Where("EXISTS (SELECT 1 FROM weibo_chat_bookmark_message m WHERE m.block_id = weibo_chat_bookmark_block.id AND m.group_id = ? AND m.message_id = ?)", filter.Message.GroupID, filter.Message.MessageID)
	}
	if filter.Cursor != "" {
		var c bookmarkBlockCursor
		if err := decodeBookmarkCursor(filter.Cursor, &c); err != nil {
			return page, err
		}
		if _, err := uuid.Parse(c.ID); err != nil || c.UpdatedAt.IsZero() {
			return page, ErrInvalidBookmarkCursor
		}
		q = q.Where("(updated_at, id) < (?, ?::uuid)", c.UpdatedAt, c.ID)
	}
	if err := q.Order("updated_at DESC, id DESC").Limit(filter.Limit + 1).Find(&page.Blocks).Error; err != nil {
		return page, err
	}
	if len(page.Blocks) > filter.Limit {
		page.Blocks = page.Blocks[:filter.Limit]
		last := page.Blocks[len(page.Blocks)-1]
		page.NextCursor = encodeBookmarkCursor(bookmarkBlockCursor{last.UpdatedAt, last.ID})
	}
	return page, s.populateBookmarkBlocks(ctx, page.Blocks)
}
func (s *PostgresStore) ListBookmarkMessages(ctx context.Context, owner, id, cursor string, limit int) (BookmarkMessagePage, error) {
	page := BookmarkMessagePage{Messages: []Message{}}
	if limit < 1 || limit > 100 {
		return page, fmt.Errorf("limit must be between 1 and 100")
	}
	if _, err := findBookmark(s.db.WithContext(ctx), owner, id, false); err != nil {
		return page, err
	}
	q := s.db.WithContext(ctx).Model(&Message{}).Omit("Raw").Where("EXISTS (SELECT 1 FROM weibo_chat_bookmark_message b WHERE b.block_id = ? AND b.group_id = weibo_chat_message.group_id AND b.message_id = weibo_chat_message.message_id)", id)
	if cursor != "" {
		var c bookmarkMessageCursor
		if err := decodeBookmarkCursor(cursor, &c); err != nil {
			return page, err
		}
		if !MessageID(c.MessageID).Valid() || !MessageID(c.GroupID).Valid() {
			return page, ErrInvalidBookmarkCursor
		}
		tail := `(length(message_id), message_id COLLATE "C", length(group_id), group_id COLLATE "C") > (?, ? COLLATE "C", ?, ? COLLATE "C")`
		args := []any{len(c.MessageID), c.MessageID, len(c.GroupID), c.GroupID}
		if c.SentAt == nil {
			q = q.Where("sent_at IS NULL AND "+tail, args...)
		} else {
			q = q.Where("sent_at IS NULL OR sent_at > ? OR (sent_at = ? AND "+tail+")", append([]any{*c.SentAt, *c.SentAt}, args...)...)
		}
	}
	if err := q.Order(`sent_at ASC NULLS LAST, length(message_id), message_id COLLATE "C", length(group_id), group_id COLLATE "C"`).Limit(limit + 1).Find(&page.Messages).Error; err != nil {
		return page, err
	}
	if len(page.Messages) > limit {
		page.Messages = page.Messages[:limit]
		last := page.Messages[len(page.Messages)-1]
		page.NextCursor = encodeBookmarkCursor(bookmarkMessageCursor{last.SentAt, last.GroupID, string(last.MessageID)})
	}
	return page, nil
}
func (s *PostgresStore) ListBookmarkTags(ctx context.Context, owner string) ([]string, error) {
	tags := []string{}
	err := s.db.WithContext(ctx).Model(&BookmarkTag{}).Distinct("name").Where("block_id IN (?)", s.db.Model(&BookmarkBlock{}).Select("id").Where("owner_username = ?", owner)).Order("name").Pluck("name", &tags).Error
	return tags, err
}
func (s *PostgresStore) GetBookmarkCounts(ctx context.Context, owner, group string, ids []string) (map[string]int64, error) {
	counts := map[string]int64{}
	if len(ids) == 0 {
		return counts, nil
	}
	var rows []struct {
		MessageID string
		Count     int64
	}
	err := s.db.WithContext(ctx).Model(&BookmarkMessage{}).Select("message_id, count(*) AS count").Where("group_id = ? AND message_id IN ?", group, ids).Where("block_id IN (?)", s.db.Model(&BookmarkBlock{}).Select("id").Where("owner_username = ?", owner)).Group("message_id").Scan(&rows).Error
	for _, row := range rows {
		counts[row.MessageID] = row.Count
	}
	return counts, err
}
