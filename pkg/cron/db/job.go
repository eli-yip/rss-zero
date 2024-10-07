package db

import (
	"errors"
	"time"

	"github.com/rs/xid"
	"gorm.io/gorm"
)

type CronJob struct {
	ID        string    `gorm:"primaryKey;column:id;type:string" json:"id"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	TaskType  string    `gorm:"column:task_type;type:string" json:"task_type"` // definition id
	Status    int       `gorm:"column:status;type:int" json:"status"`
	Detail    string    `gorm:"column:detail;type:string" json:"detail"`
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
	AddJob(jobID, taskType string) (job *CronJob, err error)
	StopJob(jobID string) (err error)
	CheckRunningJob(taskType string) (job *CronJob, err error)
	FindRunningJob() ([]*CronJob, error)
	FindErrorJob() ([]*CronJob, error)
	UpdateStatus(jobID string, status int) (err error)
	RecordDetail(jobID, detail string) (err error)
}

func (ds *DBService) AddJob(jobID, taskType string) (job *CronJob, err error) {
	if jobID == "" {
		jobID = xid.New().String()
	}
	job = &CronJob{
		ID:       jobID,
		TaskType: taskType,
		Status:   StatusRunning}
	err = ds.Save(job).Error
	return job, err
}

func (ds *DBService) CheckRunningJob(taskType string) (job *CronJob, err error) {
	job = &CronJob{}
	err = ds.Where("task_type = ? AND status = ?", taskType, StatusRunning).First(&job).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}

func (ds *DBService) FindRunningJob() (jobs []*CronJob, err error) {
	err = ds.Where("status = ?", StatusRunning).Find(&jobs).Error
	return jobs, err
}

func (ds *DBService) FindErrorJob() (jobs []*CronJob, err error) {
	err = ds.Where("status = ?", StatusError).Find(&jobs).Error
	return jobs, err
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
