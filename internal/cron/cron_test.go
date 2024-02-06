package cron

import (
	"os"
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/log"
	"github.com/go-co-op/gocron/v2"
	"go.uber.org/zap"
)

func TestZsxq(t *testing.T) {
	t.Log("TestZsxq")
	config.InitFromEnv()

	redisService, _ := redis.NewRedisService(config.C.Redis)
	db, _ := db.NewPostgresDB(config.C.DB)

	logger := log.NewLogger()
	bark := notify.NewBarkNotifier(os.Getenv("BARK_URL"))
	zsxqCrawler := CrawlZsxq(redisService, db, bark)

	location, _ := time.LoadLocation("Asia/Shanghai")
	s, err := gocron.NewScheduler(gocron.WithLocation(location))
	if err != nil {
		t.Fatal(err)
	}

	j, err := s.NewJob(
		gocron.OneTimeJob(gocron.OneTimeJobStartImmediately()),
		gocron.NewTask(zsxqCrawler),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		t.Fatal(err)
	}

	logger.Info("add job", zap.Any("job", j.ID()))
	s.Start()

	<-time.After(time.Minute * 10)
}
