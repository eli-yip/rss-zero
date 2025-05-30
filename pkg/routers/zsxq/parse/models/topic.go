// This file defines the struct of api response struct of zsxq.
// Attention: the struct is api response struct, not the struct of database.
package models

import "encoding/json"

type TopicParseResult struct {
	Topic
	AuthorID   int
	AuthorName string
	Text       string // Markdown main body
	Raw        json.RawMessage
}

type Topic struct {
	TopicID int `json:"topic_id"`
	Group   struct {
		GroupID int    `json:"group_id"`
		Name    string `json:"name"`
	} `json:"group"`
	Type       string    `json:"type"`
	CreateTime string    `json:"create_time"`
	Talk       *Talk     `json:"talk"`
	Question   *Question `json:"question"`
	Answer     *Answer   `json:"answer"`
	Title      *string   `json:"title"`
	Digested   bool      `json:"digested"`
}

type Talk struct {
	Owner   User     `json:"owner"`
	Text    *string  `json:"text"`
	Article *Article `json:"article"`
	Files   []File   `json:"files"`
	Images  []Image  `json:"images"`
}

type Article struct {
	Title      string `json:"title"`
	ArticleID  string `json:"article_id"`
	ArticleURL string `json:"article_url"`
}

type File struct {
	FileID int    `json:"file_id"`
	Name   string `json:"name"`
}

type Question struct {
	Questionee User    `json:"questionee"`
	Text       string  `json:"text"`
	Images     []Image `json:"images"`
}

type Answer struct {
	Answerer User    `json:"owner"`
	Text     *string `json:"text"`
	Voice    *Voice  `json:"voice"`
	Images   []Image `json:"images"`
}

type Image struct {
	ImageID   int    `json:"image_id"`
	Type      string `json:"type"`
	Thumbnail *struct {
		URL string `json:"url"`
	} `json:"thumbnail"`
	Large *struct {
		URL string `json:"url"`
	} `json:"large"`
	Original *struct {
		URL string `json:"url"`
	} `json:"original"`
}

type User struct {
	UserID int     `json:"user_id"`
	Name   string  `json:"name"`
	Alias  *string `json:"alias"`
}

type Voice struct {
	VoiceID int    `json:"voice_id"`
	URL     string `json:"url"`
}
