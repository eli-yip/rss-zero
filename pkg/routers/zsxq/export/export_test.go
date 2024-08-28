package export

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	render "github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
)

type mockZsxqDBService struct{ zsxqDB.DB }

func (m *mockZsxqDBService) GetAuthorID(name string) (int, error) {
	const authorID = 11111
	if name == "author" {
		return authorID, nil
	}
	return 0, gorm.ErrRecordNotFound
}

func (m *mockZsxqDBService) FetchNTopics(groupID int, opt zsxqDB.Options) ([]zsxqDB.Topic, error) {
	return []zsxqDB.Topic{
		{
			ID:        1,
			Time:      time.Date(2022, 11, 20, 0, 0, 0, 0, time.Local),
			GroupID:   28855218411241,
			Type:      "q&a",
			Digested:  true,
			AuthorID:  11111,
			ShareLink: "https://wx.zsxq.com/dweb2/index/topic/28855218411241",
			Title:     func() *string { s := "title"; return &s }(),
			Text:      "text",
		},
		{
			ID:        22222,
			Time:      time.Date(2022, 11, 20, 0, 0, 0, 0, time.Local),
			GroupID:   28855218411241,
			Type:      "q&a",
			Digested:  false,
			AuthorID:  11111,
			ShareLink: "https://wx.zsxq.com/dweb2/index/topic/28855218411241",
			Title:     func() *string { s := "title2"; return &s }(),
			Text:      "text",
		},
	}, nil
}

func TestExport(t *testing.T) {
	t.Log("TestExport")

	type testCase struct {
		option Option
		expect string
	}

	testCases := []testCase{
		{
			option: Option{},
			expect: `# [精华]title

时间：2022年11月20日

链接：[https://wx.zsxq.com/dweb2/index/topic/28855218411241](https://wx.zsxq.com/dweb2/index/topic/28855218411241)

text

# title2

时间：2022年11月20日

链接：[https://wx.zsxq.com/dweb2/index/topic/28855218411241](https://wx.zsxq.com/dweb2/index/topic/28855218411241)

text
`,
		},
	}

	assert := assert.New(t)

	zsxqDB := &mockZsxqDBService{}
	exportService := NewExportService(zsxqDB, render.NewMarkdownRenderService(zsxqDB))

	var buf bytes.Buffer
	for _, v := range testCases {
		err := exportService.Export(&buf, v.option)
		assert.Nil(err)
		assert.Equal(v.expect, buf.String())
	}

	t.Log("TestExport done")
}

func TestExportOptionError(t *testing.T) {
	t.Log("TestExportOptionError")

	type testCase struct {
		option Option
		err    error
	}

	testCases := []testCase{
		{
			option: Option{
				AuthorName: func() *string { s := "author_abc"; return &s }(),
			},
			err: ErrNoAuthor,
		},
		{
			option: Option{
				StartTime: time.Date(2022, 11, 20, 0, 0, 0, 0, time.Local),
				EndTime:   time.Date(2022, 11, 19, 0, 0, 0, 0, time.Local),
			},
			err: ErrTimeOrder,
		},
	}

	assert := assert.New(t)

	zsxqDB := &mockZsxqDBService{}
	exportService := NewExportService(zsxqDB, render.NewMarkdownRenderService(zsxqDB))

	for _, v := range testCases {
		var buf bytes.Buffer
		err := exportService.Export(&buf, v.option)
		assert.Equal(v.err, err)
	}

	t.Log("TestExportOptionError done")
}

func TestFileName(t *testing.T) {
	exportService := ExportService{}

	options := []struct {
		Option Option
		Expect string
	}{
		{
			Option: Option{
				GroupID:    28855218411241,
				Type:       nil,
				Digested:   nil,
				AuthorName: nil,
				StartTime:  time.Date(2022, 11, 20, 0, 0, 0, 0, time.Local),
				EndTime:    time.Date(2022, 11, 25, 0, 0, 0, 0, time.Local),
			},
			Expect: "知识星球合集-28855218411241-2022-11-20-2022-11-24.md",
		},
		{
			Option: Option{
				GroupID:    28855218411241,
				Type:       func() *string { s := "q&a"; return &s }(),
				Digested:   func() *bool { b := true; return &b }(),
				AuthorName: nil,
				StartTime:  time.Date(2022, 11, 20, 0, 0, 0, 0, time.Local),
				EndTime:    time.Date(2022, 11, 25, 0, 0, 0, 0, time.Local),
			},
			Expect: "知识星球合集-28855218411241-q&a-digest-2022-11-20-2022-11-24.md",
		},
	}

	assert := assert.New(t)

	for _, v := range options {
		got := exportService.FileName(v.Option)
		assert.Equal(v.Expect, got)
	}
}
