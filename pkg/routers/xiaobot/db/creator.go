package db

import "fmt"

type Creator struct {
	ID       string `gorm:"column:id;type:text;primaryKey"`
	NickName string `gorm:"column:nickname;type:text"`
}

func (c *Creator) TableName() string { return "xiaobot_creator" }

type DBCreator interface {
	// SaveCreator save a xiaobot creator to db
	SaveCreator(creator *Creator) (err error)
	// GetCreatorName get a xiaobot creator name from db by id
	GetCreatorName(id string) (string, error)
}

func (d *DBService) SaveCreator(creator *Creator) (err error) { return d.Save(creator).Error }

func (d *DBService) GetCreatorName(id string) (string, error) {
	var creator Creator
	if err := d.Where("id = ?", id).First(&creator).Error; err != nil {
		return "", fmt.Errorf("failed to get creator %s: %w", id, err)
	}
	return creator.NickName, nil
}
