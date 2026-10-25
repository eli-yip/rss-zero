package weibochat

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// User holds the most recently observed profile. The avatar source and stored
// object always describe the same successfully archived image.
type User struct {
	UID                   string    `gorm:"type:text;primaryKey"`
	ScreenName            string    `gorm:"type:text;index"`
	AvatarSourceURL       string    `gorm:"type:text"`
	AvatarObjectKey       string    `gorm:"type:text"`
	AvatarStorageProvider string    `gorm:"type:text"`
	ProfileObservedAt     time.Time `gorm:"type:timestamptz;not null"`
}

func (User) TableName() string { return "weibo_chat_user" }

type UserStore interface {
	GetUser(context.Context, string) (*User, error)
	SaveUser(context.Context, *User) error
}

func (s *PostgresStore) GetUser(ctx context.Context, uid string) (*User, error) {
	var user User
	err := s.db.WithContext(ctx).Where("uid = ?", uid).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *PostgresStore) SaveUser(ctx context.Context, user *User) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "uid"}},
		DoUpdates: clause.AssignmentColumns([]string{"screen_name", "avatar_source_url", "avatar_object_key", "avatar_storage_provider", "profile_observed_at"}),
		Where:     clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "weibo_chat_user.profile_observed_at <= EXCLUDED.profile_observed_at"}}},
	}).Create(user).Error
}

// UserArchiver refreshes profiles and archives changed avatars while fetching
// messages. It does not change existing messages or maintain profile history.
type UserArchiver struct {
	client *Client
	store  UserStore
	files  ObjectStore
}

func NewUserArchiver(client *Client, store UserStore, files ObjectStore) *UserArchiver {
	return &UserArchiver{client: client, store: store, files: files}
}

// SyncUsers observes every sender on the page, including already archived messages.
// Failed avatars leave their previous source/object pair intact and return errors;
// other senders on the same page are still processed.
func (a *UserArchiver) SyncUsers(ctx context.Context, messages []Message) error {
	profiles := map[string]User{}
	for _, message := range messages {
		if message.Sender == nil {
			continue
		}
		incoming := *message.Sender
		incoming.ProfileObservedAt = message.FetchedAt
		previous, exists := profiles[incoming.UID]
		if exists && incoming.ProfileObservedAt.Before(previous.ProfileObservedAt) {
			continue
		}
		if incoming.ScreenName == "" {
			incoming.ScreenName = previous.ScreenName
		}
		if incoming.AvatarSourceURL == "" {
			incoming.AvatarSourceURL = previous.AvatarSourceURL
		}
		profiles[incoming.UID] = incoming
	}
	keys := make([]string, 0, len(profiles))
	for uid := range profiles {
		keys = append(keys, uid)
	}
	sort.Strings(keys)
	var failures []error
	for _, uid := range keys {
		if err := a.syncUser(ctx, profiles[uid]); err != nil {
			failures = append(failures, fmt.Errorf("user %s: %w", uid, err))
		}
	}
	return errors.Join(failures...)
}

func (a *UserArchiver) syncUser(ctx context.Context, profile User) error {
	current, err := a.store.GetUser(ctx, profile.UID)
	if err != nil {
		return err
	}
	if current == nil {
		current = &User{UID: profile.UID}
	}
	if profile.ProfileObservedAt.Before(current.ProfileObservedAt) {
		return nil
	}
	if profile.ScreenName != "" {
		current.ScreenName = profile.ScreenName
	}
	current.ProfileObservedAt = profile.ProfileObservedAt
	var avatarErr error
	if profile.AvatarSourceURL != "" {
		if avatarIdentity(profile.AvatarSourceURL) != avatarIdentity(current.AvatarSourceURL) || current.AvatarObjectKey == "" {
			avatarErr = a.archiveAvatar(ctx, current, profile.AvatarSourceURL)
		} else {
			// Refresh the full signed source URL without downloading unchanged pixels.
			current.AvatarSourceURL = profile.AvatarSourceURL
		}
	}
	return errors.Join(avatarErr, a.store.SaveUser(ctx, current))
}

func (a *UserArchiver) archiveAvatar(ctx context.Context, user *User, source string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	keyPrefix := fmt.Sprintf("weibo-chat/users/%s/%x", user.UID, sha256.Sum256([]byte(avatarIdentity(source))))
	stored, err := a.client.storeMedia(ctx, a.files, source, "image", keyPrefix)
	if err != nil {
		return fmt.Errorf("archive avatar: %w", err)
	}
	user.AvatarSourceURL = source
	user.AvatarObjectKey = stored.key
	user.AvatarStorageProvider = a.files.AssetsDomain()
	return nil
}

// Sina rotates these access parameters even when the avatar itself is unchanged.
// Other hosts and query parameters retain their original identity semantics.
func avatarIdentity(source string) string {
	u, err := url.Parse(source)
	if err != nil || (u.Hostname() != "sinaimg.cn" && !strings.HasSuffix(u.Hostname(), ".sinaimg.cn")) {
		return source
	}
	query := u.Query()
	query.Del("Expires")
	query.Del("ssig")
	u.RawQuery = query.Encode()
	return u.String()
}

// GetUsers loads the latest profiles for a message page in one query.
func (s *PostgresStore) GetUsers(ctx context.Context, uids []string) ([]User, error) {
	users := []User{}
	if len(uids) == 0 {
		return users, nil
	}
	err := s.db.WithContext(ctx).Where("uid IN ?", uids).Find(&users).Error
	return users, err
}

// ListSenders lists observed senders, not current group membership. A sender
// whose profile has not been archived is still represented by their UID.
func (s *PostgresStore) ListSenders(ctx context.Context, groupID string) ([]User, error) {
	users := []User{}
	err := s.db.WithContext(ctx).Raw(`
		SELECT senders.sender_id AS uid, users.screen_name, users.avatar_source_url,
			users.avatar_object_key, users.avatar_storage_provider, users.profile_observed_at
		FROM (SELECT DISTINCT sender_id FROM weibo_chat_message WHERE group_id = ? AND sender_id <> '') AS senders
		LEFT JOIN weibo_chat_user AS users ON users.uid = senders.sender_id
		ORDER BY length(senders.sender_id), senders.sender_id COLLATE "C"
	`, groupID).Scan(&users).Error
	return users, err
}
