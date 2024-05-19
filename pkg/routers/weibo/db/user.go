package db

import (
	"errors"

	"gorm.io/gorm"
)

type User struct {
	ID       int    `gorm:"column:id;type:int;primary_key"`
	Nickname string `gorm:"column:nickname;type:text"`
}

func (u *User) TableName() string { return "weibo_user" }

type DBUser interface {
	SaveUser(u *User) (err error)
	GetUser(id int) (u *User, err error)
}

func (d *DBService) SaveUser(u *User) (err error) { return d.Save(u).Error }

var ErrUserNotExist = errors.New("user not exist")

func (d *DBService) GetUser(id int) (u *User, err error) {
	u = &User{}
	if err = d.Where("id = ?", id).First(u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotExist
		}
		return nil, err
	}
	return u, err
}
