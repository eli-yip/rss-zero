//lint:file-ignore U1000 Ignore all unused code in development
package render

import (
	"strings"

	"github.com/eli-yip/rss-zero/internal/md"
)

type RenderService struct{}

func NewRenderService() *RenderService { return &RenderService{} }

func (rs *RenderService) renderPicInfos(picInfos []PicInfo) (text string) {
	for _, picInfo := range picInfos {
		text += md.Image(picInfo.ObjectKey, picInfo.URL) + "\n\n"
	}
	return trimRightNewLine(text)
}

func trimRightNewLine(text string) string { return strings.TrimRight(text, "\n") }
