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
	if err = s.Del(cookieType); err != nil {
		return fmt.Errorf("failed to delete cookie: %w", err)
	}

	if err = s.Check(cookieType); !errors.Is(err, ErrKeyNotExist) {
		return fmt.Errorf("cookie already exists or some other error: %w", err)
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
	err = s.Where("type = ? AND expire_at >= ?", cookieType, time.Now()).Debug().First(&c).Error
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

func (s *CookieService) CheckTTL(cookieType int, ttl time.Duration) (err error) {
	var count int64
	err = s.Model(&Cookie{}).Where("type = ? AND expire_at >= ?", cookieType, time.Now().Add(ttl)).Count(&count).Error
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

func (s *CookieService) GetCookieTypes() (cookieTypes []int, err error) {
	err = s.Model(&Cookie{}).Select("DISTINCT type").Pluck("type", &cookieTypes).Error
	return cookieTypes, err
}
