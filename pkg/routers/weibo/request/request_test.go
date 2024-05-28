package request

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/redis"
)

func TestRequestService(t *testing.T) {
	assert := assert.New(t)
	assert.Nil(config.InitForTestToml())
	cookie := `XSRF-TOKEN=_O9l0fMy--6L9QOAA7_cxaGc; SCF=Ah80diSfdjZZKoQUcS729bBHzvndNucLFqhboeznHssu6UrsMvZQks0KFb80YxAWvF38aAnVMiXMkkCZOJNoxn0.; SSOLoginState=1704025971; SUBP=0033WrSXqPxfM725Ws9jqgMF55529P9D9WFsN6S3FPT6QJP8oOV_.9SB5JpX5KMhUgL.FoMXSoBc1hncSKz2dJLoI7piC-LiC-LiC-Ljgg-t; _s_tentry=weibo.com; Apache=9211835701436.395.1710839832750; SINAGLOBAL=9211835701436.395.1710839832750; ULV=1710839832785:1:1:1:9211835701436.395.1710839832750:; ALF=1717590299; SUB=_2A25LPLxLDeRhGeFK7VYX-CbKzj6IHXVoM7GDrDV8PUJbkNANLVKgkW1NQvjxC1-NyMQhak8FnveQyGjhDPW1gi1U; WBPSESS=q93jtNaJV_9UuQMNTwgngvboWohnOiuUAjyzsCeWdG9Qgx0xUhLRhApDo25tMrPNbbo6G_weeXBnu8b51I1OQ0kLME-u0dduHvih0EyUZWK-3W677n6jZU_8QMAxc-gt3rh0XY1lWc7Mpu_ZlJ0fbA==`
	redisService, err := redis.NewRedisService(config.C.Redis)
	assert.Nil(err)
	rs, err := NewRequestService(redisService, cookie, log.NewZapLogger())
	assert.Nil(err)
	t.Run("Limit raw", func(t *testing.T) {
		data, err := rs.LimitRaw("https://weibo.com/ajax/statuses/searchProfile?uid=6827625527&page=1&hasori=1&hastext=1&haspic=1&hasvideo=1&hasmusic=1&hasret=1&sudaref=weibo.com&display=0&retcode=6102")
		assert.Nil(err)
		fmt.Println(string(data))
	})

	t.Run("Download pictures", func(t *testing.T) {
		resp, err := rs.GetPicStream("https://wx3.sinaimg.cn/orj1080/007s41inly1hpfw0vi7lvj30j30rs417.jpg")
		assert.Nil(err)
		assert.NotNil(resp)
		bytes, err := io.ReadAll(resp.Body)
		assert.Nil(err)
		file, err := os.OpenFile("test.jpg", os.O_CREATE|os.O_WRONLY, 0644)
		assert.Nil(err)
		_, err = file.Write(bytes)
		assert.Nil(err)
	})
}
