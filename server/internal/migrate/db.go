package migrate

import (
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/pkg/cookie"
	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
	githubDB "github.com/eli-yip/rss-zero/pkg/routers/github/db"
	"github.com/eli-yip/rss-zero/pkg/routers/macked"
	weiboDB "github.com/eli-yip/rss-zero/pkg/routers/weibo/db"
	xiaobotDB "github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
)

func MigrateDB(db *gorm.DB) (err error) {
	return db.AutoMigrate(
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
		&zhihuDB.EncryptionService{},
		&zhihuDB.Zvideo{},

		&xiaobotDB.Paper{},
		&xiaobotDB.Post{},
		&xiaobotDB.Creator{},

		&weiboDB.Tweet{},
		&weiboDB.Object{},
		&weiboDB.User{},

		&cronDB.CronTask{},
		&cronDB.CronJob{},

		&githubDB.Release{},
		&githubDB.Sub{},
		&githubDB.Repo{},

		&macked.TimeInfo{},
		&macked.AppInfo{},

		&cookie.Cookie{},
	)
}
