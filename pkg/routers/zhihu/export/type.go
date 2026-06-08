package export

import (
	"errors"
	"fmt"

	"github.com/eli-yip/rss-zero/pkg/common"
)

func (opt Option) ZhihuContentType() (common.ZhihuContentType, error) {
	if opt.Type == nil {
		return "", errors.New("type is required")
	}

	contentType, err := common.ParseZhihuLegacyID(*opt.Type)
	if err != nil {
		return "", fmt.Errorf("unknown type: %w", err)
	}
	return contentType, nil
}
