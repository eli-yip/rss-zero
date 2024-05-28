package db

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/eli-yip/rss-zero/config"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
)

func NewPostgresDB(c config.DatabaseConfig) (db *gorm.DB, err error) {
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold: time.Second,
			LogLevel:      logger.Silent,
			Colorful:      false,
		},
	)

	mdsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
		c.Host, c.Port, c.User, c.Password, c.Name)
	if db, err = gorm.Open(postgres.Open(mdsn), &gorm.Config{
		PrepareStmt: true,
		Logger:      newLogger,
	}); err != nil {
		panic(err)
	}

	// migrate
	if err = db.AutoMigrate(
		&zsxqDB.Topic{},
		&zsxqDB.Group{},
		&zsxqDB.Author{},
		&zsxqDB.Object{},
		&zsxqDB.Article{},

		&zhihuDB.Answer{},
		&zhihuDB.Question{},
		&zhihuDB.Author{},
		&zhihuDB.Object{},
		&zhihuDB.Article{},
		&zhihuDB.Pin{},
		&zhihuDB.Sub{},

		&xiaobotDB.Paper{},
		&xiaobotDB.Post{},
		&xiaobotDB.Creator{},
	); err != nil {
		return nil, err
	}

	mdb, _ := db.DB()
	mdb.SetMaxIdleConns(20)
	mdb.SetMaxOpenConns(100)
	mdb.SetConnMaxLifetime(time.Hour)

	return db, nil
}
