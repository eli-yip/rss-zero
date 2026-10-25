package weibochat

import (
	"context"
	"fmt"

	"gorm.io/gorm/clause"
)

// UserPreference belongs to a reader, independently of the shared Weibo profile.
type UserPreference struct {
	OwnerUsername string `gorm:"type:text;primaryKey"`
	UID           string `gorm:"type:text;primaryKey"`
	IsStarred     bool   `gorm:"not null;default:false"`
	IsBlocked     bool   `gorm:"not null;default:false"`
}

func (UserPreference) TableName() string { return "weibo_chat_user_preference" }

type UserPreferencePatch struct {
	IsStarred *bool
	IsBlocked *bool
}

type UserPreferenceFilter struct {
	Starred  *bool
	Blocked  *bool
	AfterUID string
	Limit    int
}

type UserPreferencePage struct {
	Items   []UserPreference
	HasMore bool
}

func (s *PostgresStore) UpdateUserPreference(ctx context.Context, owner, uid string, patch UserPreferencePatch) (UserPreference, error) {
	result := UserPreference{OwnerUsername: owner, UID: uid}
	if owner == "" || !MessageID(uid).Valid() || (patch.IsStarred == nil && patch.IsBlocked == nil) {
		return result, fmt.Errorf("owner, valid uid and at least one preference are required")
	}
	columns := []string{}
	if patch.IsStarred != nil {
		result.IsStarred = *patch.IsStarred
		columns = append(columns, "is_starred")
	}
	if patch.IsBlocked != nil {
		result.IsBlocked = *patch.IsBlocked
		columns = append(columns, "is_blocked")
	}
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "owner_username"}, {Name: "uid"}},
		DoUpdates: clause.AssignmentColumns(columns),
	}, clause.Returning{}).Create(&result).Error
	return result, err
}

func (s *PostgresStore) GetUserPreferences(ctx context.Context, owner string, uids []string) (map[string]UserPreference, error) {
	result := make(map[string]UserPreference)
	if len(uids) == 0 {
		return result, nil
	}
	var rows []UserPreference
	err := s.db.WithContext(ctx).Where("owner_username = ? AND uid IN ?", owner, uids).Find(&rows).Error
	for _, row := range rows {
		result[row.UID] = row
	}
	return result, err
}

func (s *PostgresStore) ListUserPreferences(ctx context.Context, owner string, filter UserPreferenceFilter) (UserPreferencePage, error) {
	result := UserPreferencePage{Items: []UserPreference{}}
	if owner == "" || filter.Limit < 1 || filter.Limit > 100 {
		return result, fmt.Errorf("owner and limit between 1 and 100 are required")
	}
	query := s.db.WithContext(ctx).Where("owner_username = ?", owner).Where("is_starred OR is_blocked")
	if filter.Starred != nil {
		query = query.Where("is_starred = ?", *filter.Starred)
	}
	if filter.Blocked != nil {
		query = query.Where("is_blocked = ?", *filter.Blocked)
	}
	if filter.AfterUID != "" {
		query = query.Where(`(length(uid), uid COLLATE "C") > (?, ? COLLATE "C")`, len(filter.AfterUID), filter.AfterUID)
	}
	err := query.Order(`length(uid), uid COLLATE "C"`).Limit(filter.Limit + 1).Find(&result.Items).Error
	if len(result.Items) > filter.Limit {
		result.Items = result.Items[:filter.Limit]
		result.HasMore = true
	}
	return result, err
}
