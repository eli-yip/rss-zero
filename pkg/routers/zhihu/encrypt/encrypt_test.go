package encrypt

import (
	"io"
	"net/http"
	"testing"

	"github.com/eli-yip/rss-zero/config"
)

func TestRealReq(t *testing.T) {
	t.Log("Test real request to zhihu api")
	config.InitFromEnv()

	cookies, err := GetCookies()
	if err != nil {
		t.Fatal(err)
	}

	api := "https://www.zhihu.com/api/v4/members/canglimo/articles?offset=0&limit=20"

	var xzse96 string
	for _, cookie := range cookies {
		if cookie.Name == "d_c0" {
			t.Logf("d_c0: %s", cookie.Value)
			xzse96, err = GetXZSE96(api, cookie.Value)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	req, err := http.NewRequest("GET", api, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36 Edg/87.0.664.75")
	req.Header.Set("x-api-version", "3.0.91")
	req.Header.Set("x-zse-93", "101_3_3.0")
	req.Header.Set("x-zse-96", xzse96)
	req.Header.Set("cookie", CookiesToString(cookies))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(respData))
}

func TestGetXZSE96(t *testing.T) {
	t.Log("Test Ecrypt Zhihu xzse96")

	cookies, err := GetCookies()
	if err != nil {
		t.Error(err)
	}

	var xzse96 string
	for _, cookie := range cookies {
		if cookie.Name == "d_c0" {
			xzse96, err = GetXZSE96("https://www.zhihu.com/people/canglimo", cookie.Value)
			if err != nil {
				t.Error(err)
			}
		}
	}
	t.Log(xzse96)
}

func TestGetCookie(t *testing.T) {
	t.Log("Test GetCookie Zhihu xzse96")

	cookies, err := GetCookies()
	if err != nil {
		t.Error(err)
	}
	t.Log(cookies)
}

func TestGetPath(t *testing.T) {
	t.Log("Test GetPath")

	tests := []struct {
		url  string
		path string
	}{
		{
			url:  "https://www.zhihu.com/people/canglimo",
			path: "/people/canglimo",
		},
		{
			url:  "https://www.zhihu.com/api/v3/moments/canglimo/activities?offset=1706671953778&page_num=1",
			path: "/api/v3/moments/canglimo/activities?offset=1706671953778&page_num=1",
		},
	}

	for _, test := range tests {
		path, err := getPath(test.url)
		if err != nil {
			t.Error(err)
		}
		if path != test.path {
			t.Errorf("path should be %s, but got %s", test.path, path)
		}
	}
}

func TestCookieToString(t *testing.T) {
	cookies := []*http.Cookie{
		{Name: "cookie1", Value: "value1"},
		{Name: "cookie2", Value: "value2"},
	}

	cookieStr := CookiesToString(cookies)

	t.Log(cookieStr)
}
