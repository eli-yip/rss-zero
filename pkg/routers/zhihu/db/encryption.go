package db

import (
	"errors"
	"math/rand/v2"
	"time"

	"gorm.io/gorm"
)

type EncryptionServiceIface interface {
	SaveService(*EncryptionService) error
	UpdateService(*EncryptionService) error
	DeleteService(string) error
	GetService(id string) (*EncryptionService, error)
	MarkAvailable(id string) error
	MarkUnavailable(id string) error
	IncreaseUsedCount(id string) error
	IncreaseFailedCount(id string) error
	SelectService() (*EncryptionService, error)
}

type EncryptionService struct {
	ID          string `gorm:"column:id;type:string;primary_key"`
	Slug        string `gorm:"column:slug;type:string"`
	URL         string `gorm:"column:url;type:string"`
	IsAvailable bool   `gorm:"column:is_available;type:bool"`
	UsedCount   int    `gorm:"column:used_count;type:int"`
	FailedCount int    `gorm:"column:failed_count;type:int"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeleteAt    gorm.DeletedAt
}

func (d *DBService) TableName() string { return "zhihu_encryption_service" }

func (d *DBService) SaveService(service *EncryptionService) error {
	service.UsedCount = 1
	return d.Create(service).Error
}

func (d *DBService) UpdateService(service *EncryptionService) error { return d.Save(service).Error }

func (d *DBService) DeleteService(id string) error {
	return d.Where("id = ?", id).Delete(&EncryptionService{}).Error
}

func (d *DBService) GetService(id string) (*EncryptionService, error) {
	var service EncryptionService
	if err := d.Where("id = ?", id).First(&service).Error; err != nil {
		return nil, err
	}
	return &service, nil
}

func (d *DBService) MarkAvailable(id string) error {
	return d.Model(&EncryptionService{}).Where("id = ?", id).Update("is_available", true).Error
}

func (d *DBService) MarkUnavailable(id string) error {
	return d.Model(&EncryptionService{}).Where("id = ?", id).Update("is_available", false).Error
}

func (d *DBService) IncreaseUsedCount(id string) error {
	return d.Model(&EncryptionService{}).Where("id = ?", id).UpdateColumn("used_count", gorm.Expr("used_count + ?", 1)).Error
}

func (d *DBService) IncreaseFailedCount(id string) error {
	return d.Model(&EncryptionService{}).Where("id = ?", id).UpdateColumn("failed_count", gorm.Expr("failed_count + ?", 1)).Error
}

func (d *DBService) SelectService() (*EncryptionService, error) {
	var services []EncryptionService
	if err := d.Where("is_available = ?", true).Find(&services).Error; err != nil {
		return nil, err
	}

	if len(services) == 0 {
		return nil, errors.New("no available services")
	}

	weights := make([]float64, len(services))
	totalWeight := 0.0

	for i, service := range services {
		if service.UsedCount == 0 {
			weights[i] = 0
		} else {
			successCount := float64(service.UsedCount - service.FailedCount)
			totalCount := float64(service.UsedCount)
			weights[i] = successCount / totalCount
		}
		totalWeight += weights[i]
	}

	if totalWeight == 0 {
		return nil, errors.New("all available services have zero weight")
	}

	randomWeight := rand.Float64() * totalWeight
	cumulativeWeight := 0.0

	for i, weight := range weights {
		cumulativeWeight += weight
		if randomWeight < cumulativeWeight {
			return &services[i], nil
		}
	}

	return nil, errors.New("failed to select a service")
}
