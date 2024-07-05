package db

import "time"

type CronJob struct {
	ID        string    `gorm:"primaryKey;column:id;type:string"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
	TaskType  string    `gorm:"column:task_type;type:string"`
	Status    int       `gorm:"column:status;type:int"`
	Detail    string    `gorm:"column:detail;type:string"`
}

func (*CronJob) TableName() string { return "cron_jobs" }

const (
	StatusPending = iota
	StatusRunning
	StatusStopped
	StatusError
	StatusFinished
)

type CronJobIface interface {
	AddJob(taskType string) (taskID string, err error)
	StopJob(taskID string) (err error)
	UpdateStatus(taskID string, status int) (err error)
	RecordDetail(taskID, detail string) (err error)
}
