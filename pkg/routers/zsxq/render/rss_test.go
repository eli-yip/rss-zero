package render

import (
	"testing"
	"time"
)

func TestRSSRender(t *testing.T) {
	topics := []RSSTopic{
		{
			TopicID:    12344,
			GroupName:  "test",
			GroupID:    8888999,
			Title:      func(s string) *string { return &s }("123"),
			AuthorName: "test-user",
			ShareLink:  "https://google.com",
			CreateTime: func(s string) time.Time {
				t, _ := time.Parse(time.RFC3339, s)
				return t
			}("2020-01-08T00:00:00+08:00"),
			Text: `> this is a question
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
			TopicID:    2,
			GroupName:  "test",
			GroupID:    8888999,
			Title:      func(s string) *string { return &s }("222"),
			AuthorName: "test-user",
			ShareLink:  "https://google.com",
			CreateTime: func(s string) time.Time {
				t, _ := time.Parse(time.RFC3339, s)
				return t
			}("2020-01-02T00:00:00+08:00"),
			Text: `> this is a question
> 2222222222222
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

	r := NewRSSRenderService()
	result, err := r.RenderRSS(topics)
	if err != nil {
		t.Errorf("RenderRSS() error = %v", err)
	}
	t.Log(result)

	// http.HandleFunc("/abc.atom", func(w http.ResponseWriter, r *http.Request) {
	// 	w.Header().Set("Content-Type", "application/atom+xml")
	// 	w.Write([]byte(result))
	// })

	// http.ListenAndServe(":8888", nil)
}
