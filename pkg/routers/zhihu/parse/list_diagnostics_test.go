package parse

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestPayloadPreviewAndTail(t *testing.T) {
	payload := []byte("0123456789")

	assert.Equal(t, "0123", previewPayload(payload, 4))
	assert.Equal(t, "6789", tailPayload(payload, 4))
	assert.Equal(t, "0123456789", previewPayload(payload, len(payload)))
	assert.Equal(t, "0123456789", tailPayload(payload, len(payload)))
	assert.Empty(t, previewPayload(payload, 0))
	assert.Empty(t, tailPayload(payload, 0))
	assert.Empty(t, previewPayload(nil, 4))
	assert.Empty(t, tailPayload(nil, 4))
}

func TestLogListPayloadDiagnostics(t *testing.T) {
	core, observed := observer.New(zap.ErrorLevel)
	logger := zap.New(core)
	err := assert.AnError
	payload := []byte(strings.Repeat("a", listPayloadPreviewLimit+10))

	logListPayloadDiagnostics(logger, "answer", 2, payload, err)

	entries := observed.All()
	if assert.Len(t, entries, 1) {
		fields := entries[0].ContextMap()
		assert.Equal(t, "answer", fields["zhihu_list_type"])
		assert.Equal(t, int64(2), fields["list_page_index"])
		assert.Equal(t, int64(len(payload)), fields["payload_len"])
		assert.Equal(t, false, fields["payload_valid_json"])
		assert.Len(t, fields["payload_preview"], listPayloadPreviewLimit)
		assert.Len(t, fields["payload_tail"], listPayloadPreviewLimit)
		assert.Equal(t, err.Error(), fields["error"])
	}
}
