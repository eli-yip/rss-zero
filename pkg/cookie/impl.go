package cookie

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/xid"
	"gorm.io/gorm"
)

type CookieService struct{ *gorm.DB }

func NewCookieService(db *gorm.DB) CookieIface { return &CookieService{db} }

func (s *CookieService) Set(cookieType int, value string, ttl time.Duration) (err error) {
	if err = s.Check(cookieType); err != nil && !errors.Is(err, ErrKeyNotExist) {
		return fmt.Errorf("failed to check cookie: %w", err)
	}

	return s.Save(&Cookie{
		ID:         xid.New().String(),
		CookieType: cookieType,
		Value:      value,
		ExpireAt:   time.Now().Add(ttl),
	}).Error
}

func (s *CookieService) Get(cookieType int) (value string, err error) {
	var c Cookie
	err = s.Where("type = ? AND expire_at >= ?", cookieType, time.Now()).First(&c).Error
	return c.Value, err
}

func (s *CookieService) Check(cookieType int) (err error) {
	var count int64
	err = s.Model(&Cookie{}).Where("type = ? AND expire_at >= ?", cookieType, time.Now()).Count(&count).Error
	if err != nil {
		return fmt.Errorf("failed to count cookie: %w", err)
	}
	if count == 0 {
		return ErrKeyNotExist
	}
	return nil
}

func (s *CookieService) GetTTL(cookieType int) (ttl time.Duration, err error) {
	var c Cookie
	err = s.Where("type = ? AND expire_at >= ?", cookieType, time.Now()).First(&c).Error
	return time.Until(c.ExpireAt), err
}

func (s *CookieService) Del(cookieType int) (err error) {
	return s.Where("type = ?", cookieType).Delete(&Cookie{}).Error
}
