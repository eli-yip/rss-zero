package render

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/pkg/log"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	dbModels "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
)

type testStruct struct {
	name   string
	topic  Topic
	result string
}

func TestToFullText(t *testing.T) {
	cases := []testStruct{
		{
			name: "basic test",
			topic: Topic{
				ID:        1234567,
				Title:     nil,
				Digested:  false,
				Time:      time.Date(2020, 1, 1, 23, 0, 0, 0, time.UTC),
				ShareLink: "https://www.google.com",
				Text:      "test-text",
			},
			result: `# 1234567

时间：2020年1月2日

链接：[https://www.google.com](https://www.google.com)

test-text
`,
		},
		{
			name: "basic test with digested",
			topic: Topic{
				ID:        1234567,
				Title:     nil,
				Digested:  true,
				Time:      time.Date(2020, 1, 1, 23, 0, 0, 0, time.UTC),
				ShareLink: "https://www.google.com",
				Text:      "test-text",
			},
			result: `# [精华]1234567

时间：2020年1月2日

链接：[https://www.google.com](https://www.google.com)

test-text
`,
		},
		{
			name: "basic test with digested and title",
			topic: Topic{
				ID:        1234567,
				Title:     func(s string) *string { return &s }("test-title"),
				Digested:  true,
				Time:      time.Date(2020, 1, 1, 23, 0, 0, 0, time.UTC),
				ShareLink: "https://www.google.com",
				Text:      "test-text",
			},
			result: `# [精华]test-title

时间：2020年1月2日

链接：[https://www.google.com](https://www.google.com)

test-text
`,
		},
		{
			name: "basic test with title",
			topic: Topic{
				ID:        1234567,
				Title:     func(s string) *string { return &s }("test-title"),
				Digested:  false,
				Time:      time.Date(2020, 1, 1, 23, 0, 0, 0, time.UTC),
				ShareLink: "https://www.google.com",
				Text:      "test-text",
			},
			result: `# test-title

时间：2020年1月2日

链接：[https://www.google.com](https://www.google.com)

test-text
`,
		},
		{
			name: "complex test with title",
			topic: Topic{
				ID:        1234567,
				Title:     func(s string) *string { return &s }("test-title"),
				Digested:  false,
				Time:      time.Date(2020, 1, 1, 23, 0, 0, 0, time.UTC),
				ShareLink: "https://www.google.com",
				Text: `作者：test-user2

test-text

这篇文章的附件如下：

第1个文件：[test-file](https://oss.momoai.me/12456-8888.jpg)

第2个文件：[test-file2](https://oss.momoai.me/12456-8888.jpg)

这篇文章的图片如下：

第1张图片：![1234567](https://oss.momoai.me/12456-8888.jpg)

第2张图片：![1234568](https://oss.momoai.me/12456-8888.jpg)
`,
			},
			result: `# test-title

时间：2020年1月2日

链接：[https://www.google.com](https://www.google.com)

作者：test-user2

test-text

这篇文章的附件如下：

第1个文件：[test-file](https://oss.momoai.me/12456-8888.jpg)

第2个文件：[test-file2](https://oss.momoai.me/12456-8888.jpg)

这篇文章的图片如下：

第1张图片：![1234567](https://oss.momoai.me/12456-8888.jpg)

第2张图片：![1234568](https://oss.momoai.me/12456-8888.jpg)
`,
		},
	}

	mockDBService := NewMockDBService()
	logger := log.NewLogger()
	MarkdownRenderService := NewMarkdownRenderService(mockDBService, logger)

	for i, c := range cases {
		var text string
		var err error
		if text, err = MarkdownRenderService.ToFullText(&c.topic); err != nil {
			t.Logf("testing %d failed", i)
			t.Errorf("RenderMarkdown() error = %v", err)
		}
		if string(text) != c.result {
			t.Logf("testing %d failed", i)
			t.Errorf("RenderMarkdown() got =\n%q, want\n%q", text, c.result)
		}
	}
}

func TestToText(t *testing.T) {
	cases := []testStruct{
		{
			name: "basic test",
			topic: Topic{
				Type:       "talk",
				AuthorName: "test-user",
				Talk: &models.Talk{
					Text: func(s string) *string { return &s }("test-text"),
				},
			},
			result: `作者：test-user

test-text
`,
		},
		{
			name: "basic test with files and images",
			topic: Topic{
				Type:       "talk",
				AuthorName: "test-user2",
				Talk: &models.Talk{
					Text: func(s string) *string { return &s }("test-text"),
					Files: []models.File{
						{
							FileID: 1234567,
							Name:   "test-file",
						},
						{
							FileID: 1234568,
							Name:   "test-file2",
						},
					},
					Images: []models.Image{
						{
							ImageID: 1234567,
							Type:    "image",
						},
						{
							ImageID: 1234568,
							Type:    "image",
						},
					},
				},
			},
			result: `作者：test-user2

test-text

这篇文章的附件如下：

第1个文件：[test-file](https://oss.momoai.me/12456-8888.jpg)

第2个文件：[test-file2](https://oss.momoai.me/12456-8888.jpg)

这篇文章的图片如下：

第1张图片：![1234567](https://oss.momoai.me/12456-8888.jpg)

第2张图片：![1234568](https://oss.momoai.me/12456-8888.jpg)
`,
		},
		{
			name: "basic test with q&a",
			topic: Topic{
				Type:       "talk",
				AuthorName: "test-user2",
				Talk: &models.Talk{
					Text: func(s string) *string { return &s }("test-text"),
					Files: []models.File{
						{
							FileID: 1234567,
							Name:   "test-file",
						},
						{
							FileID: 1234568,
							Name:   "test-file2",
						},
					},
					Images: []models.Image{
						{
							ImageID: 1234567,
							Type:    "image",
						},
						{
							ImageID: 1234568,
							Type:    "image",
						},
					},
				},
			},
			result: `作者：test-user2

test-text

这篇文章的附件如下：

第1个文件：[test-file](https://oss.momoai.me/12456-8888.jpg)

第2个文件：[test-file2](https://oss.momoai.me/12456-8888.jpg)

这篇文章的图片如下：

第1张图片：![1234567](https://oss.momoai.me/12456-8888.jpg)

第2张图片：![1234568](https://oss.momoai.me/12456-8888.jpg)
`,
		},
		{
			name: "basic test with q&a",
			topic: Topic{
				Type:       "q&a",
				AuthorName: "test-user3",
				Question: &models.Question{
					Text: "this is a question",
					Images: []models.Image{
						{
							ImageID: 1234567,
							Type:    "image",
						},
						{
							ImageID: 1234568,
							Type:    "image",
						},
					},
				},
				Answer: &models.Answer{
					Text:  func(s string) *string { return &s }("this is an answer"),
					Voice: &models.Voice{VoiceID: 1234567},
					Images: []models.Image{
						{
							ImageID: 1234567,
							Type:    "image",
						},
						{
							ImageID: 1234568,
							Type:    "image",
						},
					},
				},
			},
			result: `> this is a question
>
> 这个提问的图片如下：
>
> 第1张图片：![1234567](https://oss.momoai.me/12456-8888.jpg)
>
> 第2张图片：![1234568](https://oss.momoai.me/12456-8888.jpg)

**test-user3**回答如下：

这个[回答](https://oss.momoai.me/12456-8888.jpg)的语音转文字结果：

test-transcript

this is an answer

这个回答的图片如下：

第1张图片：![1234567](https://oss.momoai.me/12456-8888.jpg)

第2张图片：![1234568](https://oss.momoai.me/12456-8888.jpg)
`,
		},
		{
			name: "basic test with article",
			topic: Topic{
				Type:       "talk",
				AuthorName: "test-user2",
				Talk: &models.Talk{
					Text: func(s string) *string { return &s }("test-text"),
					Files: []models.File{
						{
							FileID: 1234567,
							Name:   "test-file",
						},
						{
							FileID: 1234568,
							Name:   "test-file2",
						},
					},
					Images: []models.Image{
						{
							ImageID: 1234567,
							Type:    "image",
						},
						{
							ImageID: 1234568,
							Type:    "image",
						},
					},
					Article: &models.Article{
						Title:      "test-article",
						ArticleID:  "zsxq_article_test",
						ArticleURL: "https://www.google.com",
					},
				},
			},
			result: `作者：test-user2

test-text

这篇文章的附件如下：

第1个文件：[test-file](https://oss.momoai.me/12456-8888.jpg)

第2个文件：[test-file2](https://oss.momoai.me/12456-8888.jpg)

这篇文章的图片如下：

第1张图片：![1234567](https://oss.momoai.me/12456-8888.jpg)

第2张图片：![1234568](https://oss.momoai.me/12456-8888.jpg)

这篇文章中包含有外部文章：[test-article](https://www.google.com)

文章内容如下：

test-text
`,
		},
	}

	mockDBService := NewMockDBService()
	logger := log.NewLogger()
	MarkdownRenderService := NewMarkdownRenderService(mockDBService, logger)
	for i, c := range cases {
		var text string
		var err error
		if text, err = MarkdownRenderService.ToText(&c.topic); err != nil {
			t.Logf("testing %d failed", i)
			t.Errorf("RenderMarkdown() error = %v", err)
		}
		if string(text) != c.result {
			t.Logf("testing %d failed", i)
			t.Errorf("RenderMarkdown() got =\n%q, want\n%q", text, c.result)
		}
	}
}

func TestRenderTalk(t *testing.T) {
	type testStruct struct {
		talk   models.Talk
		author string
		result string
	}
	cases := []testStruct{
		{
			talk: models.Talk{
				Text: func(s string) *string { return &s }("test-text"),
			},
			author: "test-user",
			result: `作者：test-user

test-text

`,
		},
		{
			talk: models.Talk{
				Text: func(s string) *string { return &s }("test-text"),
				Files: []models.File{
					{
						FileID: 1234567,
						Name:   "test-file",
					},
					{
						FileID: 1234568,
						Name:   "test-file2",
					},
				},
				Images: []models.Image{
					{
						ImageID: 1234567,
						Type:    "image",
					},
					{
						ImageID: 1234568,
						Type:    "image",
					},
				},
			},
			author: "test-user2",
			result: `作者：test-user2

test-text

这篇文章的附件如下：

第1个文件：[test-file](https://oss.momoai.me/12456-8888.jpg)

第2个文件：[test-file2](https://oss.momoai.me/12456-8888.jpg)

这篇文章的图片如下：

第1张图片：![1234567](https://oss.momoai.me/12456-8888.jpg)

第2张图片：![1234568](https://oss.momoai.me/12456-8888.jpg)

`,
		},
	}

	mockDBService := NewMockDBService()
	logger := log.NewLogger()
	markdownRender := NewMarkdownRenderService(mockDBService, logger)
	markdownRenderService := markdownRender.(*MarkdownRenderService)
	for _, c := range cases {
		var buffer bytes.Buffer
		if err := markdownRenderService.renderTalk(&c.talk, c.author, &buffer); err != nil {
			t.Errorf("renderTalk() error = %v", err)
		}
		if buffer.String() != c.result {
			t.Errorf("renderTalk() got =\n%q, want\n%q", buffer.String(), c.result)
		}
		fmt.Printf("%s", buffer.String())
	}
}

type mockDBService struct{}

func (m *mockDBService) SaveTopic(*dbModels.Topic) error {
	return nil
}

func (m *mockDBService) SaveObjectInfo(*dbModels.Object) error {
	return nil
}

func (m *mockDBService) GetObjectInfo(id int) (*dbModels.Object, error) {
	return &dbModels.Object{
		ID:              1234567,
		ObjectKey:       "12456-8888.jpg",
		StorageProvider: []string{"https://oss.momoai.me"},
		Transcript:      "test-transcript",
	}, nil
}

func (m *mockDBService) GetZsxqGroupIDs() ([]int, error) {
	return []int{1234567}, nil
}

func (m *mockDBService) GetLatestTopicTime(groupID int) (time.Time, error) {
	return time.Time{}, nil
}

func (m *mockDBService) FetchNTopicsBeforeTime(groupID, n int, t time.Time) ([]dbModels.Topic, error) {
	return nil, nil
}

func (m *mockDBService) UpdateCrawlTime(groupID int, t time.Time) error {
	return nil
}

func (m *mockDBService) GetLatestNTopics(groupID, n int) ([]dbModels.Topic, error) {
	return []dbModels.Topic{
		{
			ID:      1234567,
			GroupID: 1234567,
			Type:    "talk",
		},
	}, nil
}

func (m *mockDBService) GetEarliestTopicTime(groupID int) (time.Time, error) {
	return time.Time{}, nil
}

func (m *mockDBService) GetCrawlStatus(groupID int) (bool, error) {
	return false, nil
}

func (m *mockDBService) SaveCrawlStatus(groupID int, finished bool) error {
	return nil
}

func (m *mockDBService) GetGroupName(id int) (string, error) {
	return "test-group", nil
}

func (m *mockDBService) SaveAuthorInfo(*dbModels.Author) error {
	return nil
}

func (m *mockDBService) GetAuthorName(id int) (string, error) {
	return "test-user", nil
}

func (m *mockDBService) SaveArticle(*dbModels.Article) error {
	return nil
}

func (m *mockDBService) GetArticle(id string) (*dbModels.Article, error) {
	return nil, nil
}

func (m *mockDBService) GetArticleText(id string) (string, error) {
	if id == "zsxq_article_test" {
		return "test-text", nil
	}
	return "", nil
}

func (m *mockDBService) GetAllTopicIDs(id int) ([]int, error) {
	return nil, nil
}

func (m *mockDBService) GetAuthorID(name string) (int, error) {
	return 1234567, nil
}

func (m *mockDBService) FetchNTopics(n int, opt db.Options) (ts []dbModels.Topic, err error) {
	return nil, nil
}

func NewMockDBService() *mockDBService {
	return &mockDBService{}
}
