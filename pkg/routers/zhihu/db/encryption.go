package db

import (
	"errors"
	"math/rand/v2"
	"time"

	"gorm.io/gorm"
)

type EncryptionServiceIface interface {
	GetServices() ([]EncryptionService, error)
	SaveService(*EncryptionService) error
	UpdateService(*EncryptionService) error
	DeleteService(string) error
	GetService(id string) (*EncryptionService, error)
	GetServiceBySlug(slug string) (*EncryptionService, error)
	MarkAvailable(id string) error
	MarkUnavailable(id string) error
	IncreaseUsedCount(id string) error
	IncreaseFailedCount(id string) error
	SelectService() (*EncryptionService, error)
}

type EncryptionService struct {
	ID          string         `gorm:"column:id;type:string;primary_key" json:"id"`
	Slug        string         `gorm:"column:slug;type:string;unique" json:"slug"`
	URL         string         `gorm:"column:url;type:string" json:"url"`
	IsAvailable bool           `gorm:"column:is_available;type:bool" json:"is_available"`
	UsedCount   int            `gorm:"column:used_count;type:int" json:"used_count"`
	FailedCount int            `gorm:"column:failed_count;type:int" json:"failed_count"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeleteAt    gorm.DeletedAt `json:"delete_at"`
}

var ErrSlugExists = errors.New("slug should be unique")

func (d *EncryptionService) TableName() string { return "zhihu_encryption_service" }

func (d *DBService) GetServices() ([]EncryptionService, error) {
	var services []EncryptionService
	if err := d.Find(&services).Error; err != nil {
		return nil, err
	}
	return services, nil
}

func (d *DBService) SaveService(service *EncryptionService) error {
	service.UsedCount = 1
	err := d.Create(service).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrSlugExists
	}
	return err
}

func (d *DBService) UpdateService(service *EncryptionService) error {
	err := d.Save(service).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrSlugExists
	}
	return err
}

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

func (d *DBService) GetServiceBySlug(slug string) (*EncryptionService, error) {
	var service EncryptionService
	if err := d.Where("slug = ?", slug).First(&service).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
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

var ErrNoAvailableService = errors.New("no available services")

func (d *DBService) SelectService() (*EncryptionService, error) {
	var services []EncryptionService
	if err := d.Where("is_available = ?", true).Find(&services).Error; err != nil {
		return nil, err
	}

	if len(services) == 0 {
		return nil, ErrNoAvailableService
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
