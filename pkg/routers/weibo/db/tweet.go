package db

import "time"

type Tweet struct {
	ID        int       `gorm:"column:id;type:int;primary_key"`
	MBlogID   string    `gorm:"column:mblog_id;type:text"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz"`
	AuthorID  int       `gorm:"column:author_id;type:int"`
	Text      string    `gorm:"column:text;type:text"`
	Raw       []byte    `gorm:"column:raw;type:bytea"`
}

type DBTweet interface {
	SaveTweet(t *Tweet) (err error)
	GetTweet(id int) (t *Tweet, err error)
}

func (d *DBService) SaveTweet(t *Tweet) (err error) { return d.Save(t).Error }

func (d *DBService) GetTweet(id int) (t *Tweet, err error) {
	t = &Tweet{}
	err = d.Where("id = ?", id).First(t).Error
	return t, err
}
