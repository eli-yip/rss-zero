package weibochat

import (
	"context"

	"gorm.io/gorm/clause"
)

// GroupMetadata is a locally edited name, independent of crawl coverage.
type GroupMetadata struct {
	GroupID string `gorm:"type:text;primaryKey"`
	Name    string `gorm:"type:text;not null"`
}

func (GroupMetadata) TableName() string { return "weibo_chat_group" }

func (s *PostgresStore) GetGroupNames(ctx context.Context, ids []string) (map[string]string, error) {
	names := make(map[string]string)
	if len(ids) == 0 {
		return names, nil
	}
	var rows []GroupMetadata
	err := s.db.WithContext(ctx).Where("group_id IN ?", ids).Find(&rows).Error
	for _, row := range rows {
		names[row.GroupID] = row.Name
	}
	return names, err
}

func (s *PostgresStore) SetGroupName(ctx context.Context, id, name string) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "group_id"}}, DoUpdates: clause.AssignmentColumns([]string{"name"})}).Create(&GroupMetadata{GroupID: id, Name: name}).Error
}
