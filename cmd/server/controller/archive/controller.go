package archive

import (
	"gorm.io/gorm"
)

type Controller struct{ db *gorm.DB }

func NewController(db *gorm.DB) *Controller { return &Controller{db: db} }

type RandomRequest struct {
	Platform string `json:"platform"`
	Type     string `json:"type"`
	Author   string `json:"author"`
	Count    int    `json:"count"`
}

type SelectRequest struct {
	Platform string   `json:"platform"`
	IDs      []string `json:"ids"`
}

type Response struct {
	Topics []Topic `json:"topics"`
}

type ErrResponse struct {
	Message string `json:"message"`
}

type Topic struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}
