package render

import (
	"bytes"
	"testing"

	dbModels "github.com/eli-yip/zsxq-parser/pkg/db/models"
	"github.com/eli-yip/zsxq-parser/pkg/parse/models"
)

type mockDBService struct{}

func (m *mockDBService) SaveTopic(*dbModels.Topic) error {
	return nil
}

func (m *mockDBService) SaveObject(*dbModels.Object) error {
	return nil
}

func (m *mockDBService) GetObjectInfo(id int) (*dbModels.Object, error) {
	return &dbModels.Object{
		ID:              1234567,
		ObjectKey:       "12456-8888.jpg",
		StorageProvider: []string{"oss.momoai.me"},
		Transcript:      "test-transcript",
	}, nil
}

func NewMockDBService() *mockDBService {
	return &mockDBService{}
}

func TestRenderMarkdown(t *testing.T) {
	type testStruct struct {
		topic  Topic
		result string
	}
	cases := []testStruct{
		{
			topic: Topic{
				Type:   "talk",
				Author: "test-user",
				Talk: &models.Talk{
					Text: func(s string) *string { return &s }("test-text"),
				},
			},
			result: `作者：test-user

test-text

`,
		},
		{
			topic: Topic{
				Type:   "talk",
				Author: "test-user2",
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
			topic: Topic{
				Type:   "q&a",
				Author: "test-user3",
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
	}

	mockDBService := NewMockDBService()
	MarkdownRenderService := NewMarkdownRenderService(mockDBService)
	for _, c := range cases {
		var text string
		var err error
		if text, err = MarkdownRenderService.RenderMarkdown(&c.topic); err != nil {
			t.Errorf("RenderMarkdown() error = %v", err)
		}
		if text != c.result {
			t.Errorf("RenderMarkdown() got = %v, want %v", text, c.result)
		}
		// fmt.Printf("%s", text)
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
	MarkdownRenderService := NewMarkdownRenderService(mockDBService)
	for _, c := range cases {
		var buffer bytes.Buffer
		if err := MarkdownRenderService.renderTalk(&c.talk, c.author, &buffer); err != nil {
			t.Errorf("renderTalk() error = %v", err)
		}
		if buffer.String() != c.result {
			t.Errorf("renderTalk() got = %v, want %v", buffer.String(), c.result)
		}
		// fmt.Printf("%s", buffer.String())
	}
}
