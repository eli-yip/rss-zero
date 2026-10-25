package weibochat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/stretchr/testify/require"
)

func TestReadStateHTTP(t *testing.T) {
	e, db := testController(t)
	seedMessages(t, db)
	var state httputil.Resp[core.ReadStateSnapshot]
	require.NoError(t, json.Unmarshal(get(t, e, "/groups/1/read-state", 200).Body.Bytes(), &state))
	require.Equal(t, "95", *state.Data.FirstUnreadMessageID)
	for _, tc := range []struct {
		group, body string
		status      int
	}{
		{"bad", `{"action":"advance","message_id":"95","revision":0}`, 400},
		{"999", `{"action":"advance","message_id":"95","revision":0}`, 404},
		{"1", `{}`, 400},
		{"1", `{"action":"advance","message_id":"95"}`, 400},
		{"1", `{"action":"advance","message_id":"95","revision":-1}`, 400},
		{"1", `{"action":"unknown","message_id":"95","revision":0}`, 400},
		{"1", `{"action":"advance","message_id":"bad","revision":0}`, 400},
		{"1", `{"action":"advance","message_id":"999","revision":0}`, 404},
		{"1", `{"action":"advance","message_id":"95","revision":0}`, 200},
		{"1", `{"action":"advance","message_id":"100","revision":0}`, 409},
		{"1", `{"action":"mark_unread","message_id":"95","revision":1}`, 200},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPatch, "/groups/"+tc.group+"/read-state", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		e.ServeHTTP(rec, req)
		require.Equal(t, tc.status, rec.Code, rec.Body.String())
	}
	get(t, e, "/groups/bad/read-state", 400)
	get(t, e, "/groups/999/read-state", 404)
	get(t, e, "/groups/3/read-state", 200)
}
