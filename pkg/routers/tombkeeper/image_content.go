package tombkeeper

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// ErrNotImage 表示读到的内容不是支持的图片；存储读取错误保持独立。
var ErrNotImage = errors.New("not a supported image")

// ValidateImageStream 用标准库识别前 512 字节，不验证完整像素数据。
// 成功时返回回放前缀的流，由调用方关闭；失败时关闭原流。
func ValidateImageStream(body io.ReadCloser) (io.ReadCloser, string, error) {
	prefix, err := io.ReadAll(io.LimitReader(body, 512))
	if err != nil {
		_ = body.Close()
		return nil, "", fmt.Errorf("read image prefix: %w", err)
	}
	contentType := http.DetectContentType(prefix)
	switch contentType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return &replayedImage{Reader: io.MultiReader(bytes.NewReader(prefix), body), Closer: body}, contentType, nil
	default:
		_ = body.Close()
		return nil, "", fmt.Errorf("%w: %s", ErrNotImage, contentType)
	}
}

type replayedImage struct {
	io.Reader
	io.Closer
}

// DownloadImage 为历史修复重新选择候选源；图片校验由 Requester 执行。
func DownloadImage(req Requester, picID string) (*http.Response, string, error) {
	candidates, _ := candidateURLs(picID)
	return downloadFirstAvailable(req, candidates)
}

// ImageExtension 仅用于已经通过内容校验的 MIME 类型。
func ImageExtension(contentType string) string { return extFromContentType(contentType) }
