package weibochat

import (
	"encoding/json"
	core "github.com/eli-yip/rss-zero/internal/weibochat"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/stretchr/testify/require"
)

func TestGroupsIncludeConfiguredAndArchived(t *testing.T) {
	e, db := testController(t)
	seedMessages(t, db)
	var result httputil.Resp[GroupListResponse]
	rec := get(t, e, "/groups", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Len(t, result.Data.Items, 4)
	require.Equal(t, "1", result.Data.Items[0].GroupID)
	require.True(t, result.Data.Items[0].Configured)
	require.NotNil(t, result.Data.Items[0].LastSyncedAt)
	require.Equal(t, "2", result.Data.Items[1].GroupID)
	require.False(t, result.Data.Items[1].Configured)
	require.Nil(t, result.Data.Items[1].InitializedAt)
	require.Equal(t, "3", result.Data.Items[2].GroupID)
	require.True(t, result.Data.Items[2].Configured)
	require.Nil(t, result.Data.Items[2].InitializedAt)
	require.Equal(t, "4", result.Data.Items[3].GroupID)
}

func TestUpdateGroupName(t *testing.T) {
	e, db := testController(t)
	seedMessages(t, db)
	h := NewController(core.NewPostgresStore(db), []string{"1", "3"})
	e.PATCH("/groups/:group_id", h.UpdateGroup)
	for _, tc := range []struct {
		id, body string
		status   int
	}{
		{"3", `{"name":"  技术群  "}`, 200},
		{"2", `{"name":"已存档群"}`, 200},
		{"999", `{"name":"未知"}`, 404},
		{"bad", `{"name":"无效"}`, 400},
		{"1", `{}`, 400},
		{"1", `{"name":null}`, 400},
		{"1", `{"name":"` + strings.Repeat("长", 101) + `"}`, 400},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPatch, "/groups/"+tc.id, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		e.ServeHTTP(rec, req)
		require.Equal(t, tc.status, rec.Code, rec.Body.String())
	}
	var result httputil.Resp[GroupListResponse]
	require.NoError(t, json.Unmarshal(get(t, e, "/groups", 200).Body.Bytes(), &result))
	require.Equal(t, "技术群", result.Data.Items[2].Name)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/groups/3", strings.NewReader(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	e.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
	names, err := core.NewPostgresStore(db).GetGroupNames(t.Context(), []string{"3"})
	require.NoError(t, err)
	require.Empty(t, names["3"])
}
