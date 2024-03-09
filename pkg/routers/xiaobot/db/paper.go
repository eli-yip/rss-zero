package db

import (
	"errors"

	"gorm.io/gorm"
)

type Paper struct {
	ID        string `gorm:"column:id;type:text;primaryKey"`
	Name      string `gorm:"column:name;type:text"`
	CreatorID string `gorm:"column:creator_id;type:text"`
	Intro     string `gorm:"column:intro;type:text"`
}

func (p *Paper) TableName() string { return "xiaobot_paper" }

type DBPaper interface {
	// GetPapers get all xiaobot paper info in db
	GetPapers() ([]Paper, error)
	// SavePaper save a xiaobot paper info to db
	SavePaper(paper *Paper) (err error)
	// GetPaper get a xiaobot paper info from db by id
	GetPaper(id string) (*Paper, error)
	// CheckPaper check if a xiaobot paper exists in db by id
	CheckPaper(id string) (exsit bool, err error)
}

func (d *DBService) GetPapers() (papers []Paper, err error) {
	if err = d.Find(&papers).Error; err != nil {
		return nil, err
	}
	return papers, nil
}

func (d *DBService) SavePaper(paper *Paper) (err error) { return d.Save(paper).Error }

func (d *DBService) GetPaper(id string) (paper *Paper, err error) {
	paper = new(Paper)
	if err = d.Where("id = ?", id).First(paper).Error; err != nil {
		return nil, err
	}
	return paper, nil
}

func (d *DBService) CheckPaper(id string) (exist bool, err error) {
	var paper Paper
	if err = d.Where("id = ?", id).First(&paper).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
