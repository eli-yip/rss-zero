package pick

import (
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type Controller struct{ db *gorm.DB }

func NewController(db *gorm.DB) *Controller { return &Controller{db: db} }

func (h *Controller) Pick(c echo.Context) (err error) {
	return nil
}
