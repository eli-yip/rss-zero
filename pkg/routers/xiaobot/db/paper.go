package db

import (
	"errors"

	"gorm.io/gorm"
)

type Paper struct {
	ID        string `gorm:"type:text;column:id;primaryKey"`
	Name      string `gorm:"column:name;type:text"`
	CreatorID string `gorm:"column:creator_id;type:text"`
	Intro     string `gorm:"column:intro;type:text"`
}

func (p *Paper) TableName() string { return "xiaobot_paper" }

type DBPaper interface {
	GetPapers() ([]Paper, error)
	SavePaper(paper *Paper) (err error)
	GetPaper(id string) (*Paper, error)
	CheckPaper(id string) (bool, error)
}

func (d *DBService) GetPapers() ([]Paper, error) {
	var papers []Paper
	err := d.Find(&papers).Error
	return papers, err
}

func (d *DBService) SavePaper(paper *Paper) (err error) { return d.Save(paper).Error }

func (d *DBService) GetPaper(id string) (*Paper, error) {
	var paper Paper
	err := d.Where("id = ?", id).First(&paper).Error
	return &paper, err
}

func (d *DBService) CheckPaper(id string) (bool, error) {
	var paper Paper
	err := d.Where("id = ?", id).First(&paper).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
