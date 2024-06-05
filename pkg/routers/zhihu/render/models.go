package render

import "time"

type (
	// BaseContent is the common part of answer, article and pin of zhihu content
	BaseContent struct {
		ID       int
		CreateAt time.Time
		Text     string
	}

	Answer struct {
		Question BaseContent
		Answer   BaseContent
	}

	Article struct {
		Title string
		BaseContent
	}

	Pin struct{ BaseContent }
)
