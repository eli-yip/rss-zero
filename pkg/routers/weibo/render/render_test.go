package render

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderService(t *testing.T) {
	renderService := NewRenderService()
	assert := assert.New(t)

	t.Run("renderPicInfos Correctly", func(t *testing.T) {
		cases := []struct {
			picInfos []PicInfo
			want     string
		}{
			{
				picInfos: []PicInfo{
					{URL: "https://cdn.example.com/1.jpg", ObjectKey: "1.jpg"},
					{URL: "https://cdn.example.com/2.jpg", ObjectKey: "2.jpg"},
				},
				want: `![1.jpg](https://cdn.example.com/1.jpg)

![2.jpg](https://cdn.example.com/2.jpg)`,
			},
		}

		for _, c := range cases {
			assert.Equal(c.want, renderService.renderPicInfos(c.picInfos))
		}
	})
}
