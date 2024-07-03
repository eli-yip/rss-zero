package db

import "time"

type CronJob struct {
	ID        string    `gorm:"primaryKey;column:id;type:string"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
	TaskID    string    `gorm:"column:task_id;type:string"`
	Status    int       `gorm:"column:status;type:int"`
	Detail    string    `gorm:"column:detail;type:string"`
}

func (*CronJob) TableName() string { return "cron_jobs" }
