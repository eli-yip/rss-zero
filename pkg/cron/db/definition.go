package db

import (
	"errors"
	"time"

	"github.com/lib/pq"
	"github.com/rs/xid"
	"gorm.io/gorm"
)

type CronTask struct {
	ID               string    `gorm:"primaryKey;column:id;type:string"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time `gorm:"column:updated_at;autoUpdateTime"`
	DeleteAt         gorm.DeletedAt
	Type             int            `gorm:"column:type;type:int"` // zhihu, xiaobot, zsxq
	CronExpr         string         `gorm:"column:cron_expr;type:string"`
	Include          pq.StringArray `gorm:"column:include;type:text[]"`
	Exclude          pq.StringArray `gorm:"column:exclude;type:text[]"`
	CronServiceJobID string         `gorm:"column:cron_service_job_id;type:string"`
}

func (*CronTask) TableName() string { return "cron_tasks" }

type CronTaskIface interface {
	AddDefinition(taskType int, cronExpr string, include, exclude []string) (id string, err error)
	PatchDefinition(id string, cronExpr *string, include, exlucde []string, cronServiceJobID *string) (err error)
	DeleteDefinition(id string) (err error)
	GetDefinition(id string) (task *CronTask, err error)
	GetDefinitions() (tasks []*CronTask, err error)
}

const (
	TypeZsxq = iota
	TypeZhihu
	TypeXiaobot
)

func (ds *DBService) AddDefinition(taskType int, cronExpr string, include, exclude []string) (id string, err error) {
	id = xid.New().String()
	return id, ds.Save(&CronTask{
		ID:       id,
		Type:     taskType,
		CronExpr: cronExpr,
		Include:  include,
		Exclude:  exclude,
	}).Error
}

func (ds *DBService) PatchDefinition(id string, cronExpr *string, include, exclude []string, cronServiceJobID *string) (err error) {
	task := &CronTask{}
	if err = ds.First(task, "id = ?", id).Error; err != nil {
		return err
	}

	if cronExpr != nil {
		task.CronExpr = *cronExpr
	}
	if include != nil {
		task.Include = include
	}
	if exclude != nil {
		task.Exclude = exclude
	}

	if cronServiceJobID != nil {
		task.CronServiceJobID = *cronServiceJobID
	}

	return ds.Save(task).Error
}

func (ds *DBService) DeleteDefinition(id string) (err error) {
	return ds.Delete(&CronTask{}, "id = ?", id).Error
}

var (
	ErrDefinitionNotFound = errors.New("definition not found")
)

func (ds *DBService) GetDefinition(id string) (task *CronTask, err error) {
	task = &CronTask{}
	err = ds.First(task, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDefinitionNotFound
	}
	return task, err
}

func (ds *DBService) GetDefinitions() (tasks []*CronTask, err error) {
	err = ds.Find(&tasks).Error
	return tasks, err
}
