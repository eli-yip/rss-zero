package export

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/common"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zhihuRender "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

func TestExportSingle(t *testing.T) {
	assert := assert.New(t)
	config.InitFromEnv()
	db, err := db.NewPostgresDB(config.C.DB)
	assert.Nil(err)
	zhihuDBService := zhihuDB.NewDBService(db)
	fullTextRender := zhihuRender.NewFullTextRender(md.NewMarkdownFormatter())
	exportService := NewExportService(zhihuDBService, fullTextRender)

	t.Run("export single answer", func(t *testing.T) {
		file, err := os.OpenFile("test.zip", os.O_CREATE|os.O_WRONLY, 0644)
		assert.Nil(err)

		opt := Option{
			Type: func() *int {
				t := common.TypeZhihuAnswer
				return &t
			}(),
			AuthorID: func() *string {
				t := "canglimo"
				return &t
			}(),
			StartTime: time.Date(2023, 5, 1, 0, 0, 0, 0, time.UTC),
			EndTime:   time.Date(2023, 5, 5, 0, 0, 0, 0, time.UTC),
		}

		err = exportService.ExportSingle(file, opt)
		assert.Nil(err)
	})
}

func TestEscapeFilename(t *testing.T) {
	assert := assert.New(t)

	t.Run("escape filename", func(t *testing.T) {
		cases := []struct {
			filename string
			expected string
		}{
			{
				"test",
				"test",
			},
			{
				`2023-10-25-无神论者有任何理由阻止自己实施一场有利可图而确信不会被人发现/惩罚的犯罪吗？.md`,
				`2023-10-25-无神论者有任何理由阻止自己实施一场有利可图而确信不会被人发现或惩罚的犯罪吗？.md`,
			},
		}

		for _, c := range cases {
			assert.Equal(c.expected, escapeFilename(c.filename))
		}
	})
}
