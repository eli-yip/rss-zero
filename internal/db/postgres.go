package db

import (
	"fmt"
	"log"
	"os"
	"time"

	zsxqDBModels "github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDB(host, port, user, password, name string) (db *gorm.DB, err error) {
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold: time.Second,
			LogLevel:      logger.Silent,
			Colorful:      false,
		},
	)

	mdsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai", host, port, user, password, name)
	if db, err = gorm.Open(postgres.Open(mdsn), &gorm.Config{
		PrepareStmt: true,
		Logger:      newLogger,
	}); err != nil {
		panic(err)
	}

	// migrate
	if err = db.AutoMigrate(
		&zsxqDBModels.Topic{},
		&zsxqDBModels.Group{},
		&zsxqDBModels.Author{},
		&zsxqDBModels.Object{},
	); err != nil {
		panic(err)
	}

	mdb, _ := db.DB()
	mdb.SetMaxIdleConns(20)
	mdb.SetMaxOpenConns(100)
	mdb.SetConnMaxLifetime(time.Hour)

	return db, nil
}
