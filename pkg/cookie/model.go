package cookie

import "time"

type Cookie struct {
	ID         string    `gorm:"primaryKey;type:string;column:id"`
	CookieType int       `gorm:"type:int;column:type"`
	Value      string    `gorm:"type:string;column:value"`
	ExpireAt   time.Time `gorm:"type:timestamptz;column:expire_at"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (c *Cookie) TableName() string { return "cookies" }
