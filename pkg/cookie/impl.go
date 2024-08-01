package cookie

import "gorm.io/gorm"

type CookieService struct{}

func NewCookieService(db *gorm.DB) CookieIface {
	return nil //TODO
}
