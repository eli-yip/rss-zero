package db

import (
	"time"

	"github.com/rs/xid"
)

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

func (ds *DBService) AddJob(taskType string) (taskID string, err error) {
	taskID = xid.New().String()
	return taskID, ds.Save(&CronJob{
		ID:       taskID,
		TaskType: taskType,
		Status:   StatusRunning,
	}).Error
}

func (ds *DBService) StopJob(taskID string) (err error) {
	return ds.Model(&CronJob{}).Where("id = ?", taskID).Update("status", StatusStopped).Error
}

func (ds *DBService) UpdateStatus(taskID string, status int) (err error) {
	return ds.Model(&CronJob{}).Where("id = ?", taskID).Update("status", status).Error
}

func (ds *DBService) RecordDetail(taskID, detail string) (err error) {
	return ds.Model(&CronJob{}).Where("id = ?", taskID).Update("detail", detail).Error
}
