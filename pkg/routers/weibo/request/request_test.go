package request

import (
	"fmt"
	"testing"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/stretchr/testify/assert"
)

func TestRequestService(t *testing.T) {
	config.InitFromEnv()
	assert := assert.New(t)
	t.Run("Limit raw", func(t *testing.T) {
		cookie := `XSRF-TOKEN=_O9l0fMy--6L9QOAA7_cxaGc; SCF=Ah80diSfdjZZKoQUcS729bBHzvndNucLFqhboeznHssu6UrsMvZQks0KFb80YxAWvF38aAnVMiXMkkCZOJNoxn0.; SSOLoginState=1704025971; SUBP=0033WrSXqPxfM725Ws9jqgMF55529P9D9WFsN6S3FPT6QJP8oOV_.9SB5JpX5KMhUgL.FoMXSoBc1hncSKz2dJLoI7piC-LiC-LiC-Ljgg-t; _s_tentry=weibo.com; Apache=9211835701436.395.1710839832750; SINAGLOBAL=9211835701436.395.1710839832750; ULV=1710839832785:1:1:1:9211835701436.395.1710839832750:; ALF=1717590299; SUB=_2A25LPLxLDeRhGeFK7VYX-CbKzj6IHXVoM7GDrDV8PUJbkNANLVKgkW1NQvjxC1-NyMQhak8FnveQyGjhDPW1gi1U; WBPSESS=q93jtNaJV_9UuQMNTwgngvboWohnOiuUAjyzsCeWdG9Qgx0xUhLRhApDo25tMrPNbbo6G_weeXBnu8b51I1OQ0kLME-u0dduHvih0EyUZWK-3W677n6jZU_8QMAxc-gt3rh0XY1lWc7Mpu_ZlJ0fbA==`
		redisService, err := redis.NewRedisService(redis.RedisConfig{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
		})
		assert.Nil(err)
		rs, err := NewRequestService(redisService, cookie, log.NewZapLogger())
		assert.Nil(err)
		data, err := rs.LimitRaw("https://weibo.com/ajax/statuses/searchProfile?uid=6827625527&page=1&hasori=1&hastext=1&haspic=1&hasvideo=1&hasmusic=1&hasret=1&sudaref=weibo.com&display=0&retcode=6102")
		assert.Nil(err)
		fmt.Println(string(data))
	})
}
