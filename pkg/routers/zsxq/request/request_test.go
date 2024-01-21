package request

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/eli-yip/rss-zero/pkg/log"
)

func TestWithLimiterRawData(t *testing.T) {
	cookies := os.Getenv("COOKIES")
	if cookies == "" {
		t.Fatal("env COOKIES is empty")
	}
	t.Log(cookies)
	log := log.NewLogger()
	rs := NewRequestService(cookies, nil, log)
	u := "https://articles.zsxq.com/id_wsktlsarlkes.html"
	bytes, err := rs.WithLimiterRawData(u)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "raw_data.html")
	// save to file
	err = os.WriteFile(path, bytes, 0644)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(bytes))
}

func TestInvalidCookies(t *testing.T) {
	respBytes := []byte(`{
    "succeeded": false,
    "code": 401,
    "info": "",
    "resp_data": {},
    "error": "内部错误（勿告知用户、仅内部交流：签名校验未通过）"
}`)

	var respData Resp
	if err := json.Unmarshal(respBytes, &respData); err != nil {
		t.Fatal(err)
	}

	var err error
	var tooManyRequest = errors.New("too many requests")

	if respData.Succeeded {
		t.Fatal("should not be succeeded")
	} else {
		var otherResp OtherResp
		if err := json.Unmarshal(respBytes, &otherResp); err != nil {
			t.Fatal(err)
		}
		switch otherResp.Code {
		case 1059:
			err = tooManyRequest
		case 401:
			err = ErrInvalidCookie
		default:
			t.Fatal("unknown code")
		}
	}

	if err != ErrInvalidCookie {
		t.Fatal("should be too many requests")
	}
}

func TestTooManyRequests(t *testing.T) {
	respBytes := []byte(`{
    "succeeded": false,
    "code": 1059,
    "info": "",
    "resp_data": {},
    "error": "内部错误（勿告知用户、仅内部交流：签名校验未通过）"
}`)

	var respData Resp
	if err := json.Unmarshal(respBytes, &respData); err != nil {
		t.Fatal(err)
	}

	var err error
	var tooManyRequest = errors.New("too many requests")

	if respData.Succeeded {
		t.Fatal("should not be succeeded")
	} else {
		var otherResp OtherResp
		if err := json.Unmarshal(respBytes, &otherResp); err != nil {
			t.Fatal(err)
		}
		switch otherResp.Code {
		case 1059:
			err = tooManyRequest
		case 401:
			err = ErrInvalidCookie
		default:
			t.Fatal("unknown code")
		}
	}

	if err != tooManyRequest {
		t.Fatal("should be too many requests")
	}
}
