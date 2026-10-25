package weibochat

import (
	"encoding/json"
	"net/http"
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
