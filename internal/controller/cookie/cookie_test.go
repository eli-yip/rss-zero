package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/httputil"
)

type fakeStore struct {
	vals     map[int]string
	setCalls int
}

func newFakeStore() *fakeStore { return &fakeStore{vals: map[int]string{}} }

func (f *fakeStore) Set(t int, v string, _ time.Duration) error {
	f.setCalls++
	f.vals[t] = v
	return nil
}
func (f *fakeStore) Get(t int) (string, error) {
	if v, ok := f.vals[t]; ok {
		return v, nil
	}
	return "", cookie.ErrKeyNotExist
}
func (f *fakeStore) GetCookieTypes() ([]int, error) { return nil, nil }
func (f *fakeStore) Check(int) error                { return nil }
func (f *fakeStore) CheckTTL(t int, _ time.Duration) error {
	_, err := f.Get(t)
	return err
}
func (f *fakeStore) GetTTL(int) (time.Duration, error) { return time.Hour, nil }
func (f *fakeStore) Del(t int) error                   { delete(f.vals, t); return nil }
func (f *fakeStore) DelIfValue(t int, value string) (bool, error) {
	if f.vals[t] != value {
		return false, nil
	}
	delete(f.vals, t)
	return true, nil
}

type testHarness struct {
	h *Controller
	e *echo.Echo
}

func newTestHarness(store cookie.CookieIface) *testHarness {
	e := echo.New()
	e.HTTPErrorHandler = httputil.NewHTTPErrorHandler(zap.NewNop())
	return &testHarness{h: NewController(store), e: e}
}

func (th *testHarness) ctx(method, body string) (*echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := th.e.NewContext(req, rec)
	c.Set("logger", zap.NewNop())
	return c, rec
}

func renderError(e *echo.Echo, c *echo.Context, err error) {
	if err != nil {
		e.HTTPErrorHandler(c, err)
	}
}

func TestImportBrowserCookieRequiresRegisteredDomain(t *testing.T) {
	tenDays := float64(time.Now().Add(10 * 24 * time.Hour).Unix())
	tests := []struct {
		name       string
		in         InCookie
		wantStored bool
		wantReason string
	}{
		{
			name:       "matching Xiaobot domain",
			in:         InCookie{Name: "token", Value: "token=value", Domain: ".xiaobot.net", ExpirationDate: &tenDays},
			wantStored: true,
		},
		{
			name:       "same name on wrong domain",
			in:         InCookie{Name: "token", Value: "value", Domain: ".example.com", ExpirationDate: &tenDays},
			wantReason: "not registered for domain",
		},
		{
			name:       "manual GitHub token cannot be imported",
			in:         InCookie{Name: "access_token", Value: "value", Domain: ".github.com", ExpirationDate: &tenDays},
			wantReason: "not registered for domain",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore()
			result := NewController(store).importBrowserCookie(tc.in, zap.NewNop())
			if result.Stored != tc.wantStored || result.Reason != tc.wantReason {
				t.Fatalf("import result = %+v", result)
			}
			if tc.wantStored && store.vals[cookie.CookieTypeXiaobotAccessToken] != "value" {
				t.Fatalf("stored value = %q, want value", store.vals[cookie.CookieTypeXiaobotAccessToken])
			}
		})
	}
}

func TestUpdateCredentialUsesExplicitPlatformAndName(t *testing.T) {
	originalProbe := cookie.ProbeFor(cookie.CookieTypeGitHubAccessToken)
	cookie.RegisterProbe(cookie.CookieTypeGitHubAccessToken, func(value string, _ *zap.Logger) error {
		if value != "github-token" {
			t.Fatalf("probe value = %q", value)
		}
		return nil
	})
	t.Cleanup(func() { cookie.RegisterProbe(cookie.CookieTypeGitHubAccessToken, originalProbe) })

	store := newFakeStore()
	th := newTestHarness(store)
	c, rec := th.ctx(http.MethodPut, `{"value":" github-token "}`)
	c.SetPathValues(echo.PathValues{{Name: "platform", Value: "github"}, {Name: "name", Value: "access_token"}})

	renderError(th.e, c, th.h.UpdateCredential(c))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.vals[cookie.CookieTypeGitHubAccessToken] != "github-token" {
		t.Fatalf("stored value = %q", store.vals[cookie.CookieTypeGitHubAccessToken])
	}
	var response struct {
		Data CredentialUpdateResult `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.Platform != "github" || response.Data.Name != "access_token" || !response.Data.Validated {
		t.Fatalf("response = %+v", response.Data)
	}
}

func TestUpdateCredentialRejectsBrowserCookieAndInvalidToken(t *testing.T) {
	t.Run("browser cookie must use import", func(t *testing.T) {
		th := newTestHarness(newFakeStore())
		c, rec := th.ctx(http.MethodPut, `{"value":"value"}`)
		c.SetPathValues(echo.PathValues{{Name: "platform", Value: "xiaobot"}, {Name: "name", Value: "token"}})
		renderError(th.e, c, th.h.UpdateCredential(c))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid GitHub token", func(t *testing.T) {
		originalProbe := cookie.ProbeFor(cookie.CookieTypeGitHubAccessToken)
		cookie.RegisterProbe(cookie.CookieTypeGitHubAccessToken, func(string, *zap.Logger) error {
			return fmt.Errorf("%w: invalid token", cookie.ErrCredentialRejected)
		})
		t.Cleanup(func() { cookie.RegisterProbe(cookie.CookieTypeGitHubAccessToken, originalProbe) })

		store := newFakeStore()
		store.vals[cookie.CookieTypeGitHubAccessToken] = "old-token"
		th := newTestHarness(store)
		c, rec := th.ctx(http.MethodPut, `{"value":"invalid"}`)
		c.SetPathValues(echo.PathValues{{Name: "platform", Value: "github"}, {Name: "name", Value: "access_token"}})
		renderError(th.e, c, th.h.UpdateCredential(c))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if store.setCalls != 0 || store.vals[cookie.CookieTypeGitHubAccessToken] != "old-token" {
			t.Fatalf("failed validation changed stored credential: calls=%d value=%q", store.setCalls, store.vals[cookie.CookieTypeGitHubAccessToken])
		}
	})

	t.Run("GitHub validation unavailable", func(t *testing.T) {
		originalProbe := cookie.ProbeFor(cookie.CookieTypeGitHubAccessToken)
		cookie.RegisterProbe(cookie.CookieTypeGitHubAccessToken, func(string, *zap.Logger) error {
			return errors.New("upstream timeout")
		})
		t.Cleanup(func() { cookie.RegisterProbe(cookie.CookieTypeGitHubAccessToken, originalProbe) })

		store := newFakeStore()
		store.vals[cookie.CookieTypeGitHubAccessToken] = "old-token"
		th := newTestHarness(store)
		c, rec := th.ctx(http.MethodPut, `{"value":"valid-looking"}`)
		c.SetPathValues(echo.PathValues{{Name: "platform", Value: "github"}, {Name: "name", Value: "access_token"}})
		renderError(th.e, c, th.h.UpdateCredential(c))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if store.setCalls != 0 || store.vals[cookie.CookieTypeGitHubAccessToken] != "old-token" {
			t.Fatalf("unavailable validation changed stored credential: calls=%d value=%q", store.setCalls, store.vals[cookie.CookieTypeGitHubAccessToken])
		}
	})
}

func TestListCredentialsExposesUpdateMethodWithoutValue(t *testing.T) {
	store := newFakeStore()
	store.vals[cookie.CookieTypeGitHubAccessToken] = "secret"
	th := newTestHarness(store)
	c, rec := th.ctx(http.MethodGet, "")

	renderError(th.e, c, th.h.ListCredentials(c))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "secret") || !strings.Contains(body, `"update_method":"api"`) || !strings.Contains(body, `"update_method":"browser_cookie_import"`) {
		t.Fatalf("unexpected response body: %s", body)
	}
}
