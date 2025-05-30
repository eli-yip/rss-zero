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
	gormConfig := &gorm.Config{
		PrepareStmt:    true,
		TranslateError: true,
		Logger:         logger.Default,
	}
	// if !config.C.Settings.Debug {
	gormConfig.Logger = newLogger
	// }
	if db, err = gorm.Open(postgres.Open(mdsn), gormConfig); err != nil {
		panic(err)
	}

	mdb, _ := db.DB()
	mdb.SetMaxIdleConns(20)
	mdb.SetMaxOpenConns(100)
	mdb.SetConnMaxLifetime(time.Hour)

	return db, nil
}
