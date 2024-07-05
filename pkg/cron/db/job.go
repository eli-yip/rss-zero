package db

import (
	"errors"
	"time"

	"github.com/rs/xid"
	"gorm.io/gorm"
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
	AddJob(jobID, taskType string) (string, error)
	StopJob(jobID string) (err error)
	CheckJob(taskType string) (jobID string, err error)
	UpdateStatus(jobID string, status int) (err error)
	RecordDetail(jobID, detail string) (err error)
}

func (ds *DBService) AddJob(jobID, taskType string) (string, error) {
	if jobID == "" {
		jobID = xid.New().String()
	}
	return jobID, ds.Save(&CronJob{
		ID:       jobID,
		TaskType: taskType,
		Status:   StatusRunning,
	}).Error
}

func (ds *DBService) CheckJob(taskType string) (jobID string, err error) {
	var job CronJob
	err = ds.Where("task_type = ? AND status = ?", taskType, StatusRunning).First(&job).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return job.ID, nil
}

func (ds *DBService) StopJob(jobID string) (err error) {
	return ds.Model(&CronJob{}).Where("id = ?", jobID).Update("status", StatusStopped).Error
}

func (ds *DBService) UpdateStatus(jobID string, status int) (err error) {
	return ds.Model(&CronJob{}).Where("id = ?", jobID).Update("status", status).Error
}

func (ds *DBService) RecordDetail(jobID, detail string) (err error) {
	return ds.Model(&CronJob{}).Where("id = ?", jobID).Update("detail", detail).Error
}
