package parse

import (
	"encoding/json"

	"go.uber.org/zap"
)

const listPayloadPreviewLimit = 512

func logListPayloadDiagnostics(logger *zap.Logger, listType string, index int, payload []byte, err error) {
	logger.Error("Failed to unmarshal zhihu list payload",
		zap.String("zhihu_list_type", listType),
		zap.Int("list_page_index", index),
		zap.Int("payload_len", len(payload)),
		zap.Bool("payload_valid_json", json.Valid(payload)),
		zap.String("payload_preview", previewPayload(payload, listPayloadPreviewLimit)),
		zap.String("payload_tail", tailPayload(payload, listPayloadPreviewLimit)),
		zap.Error(err),
	)
}

func previewPayload(payload []byte, limit int) string {
	if limit <= 0 || len(payload) == 0 {
		return ""
	}
	if len(payload) <= limit {
		return string(payload)
	}
	return string(payload[:limit])
}

func tailPayload(payload []byte, limit int) string {
	if limit <= 0 || len(payload) == 0 {
		return ""
	}
	if len(payload) <= limit {
		return string(payload)
	}
	return string(payload[len(payload)-limit:])
}
