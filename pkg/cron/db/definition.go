package db

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

type CronTask struct {
	ID        string    `gorm:"primaryKey;column:id;type:string"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
	DeleteAt  gorm.DeletedAt
	Type      int            `gorm:"column:type;type:int"`
	CronExpr  string         `gorm:"column:cron_expr;type:string"`
	Include   pq.StringArray `gorm:"column:include;type:string[]"`
	Exclude   pq.StringArray `gorm:"column:exclude;type:string[]"`
}

func (*CronTask) TableName() string { return "cron_tasks" }
