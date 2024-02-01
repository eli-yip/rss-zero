package encrypt

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/eli-yip/rss-zero/config"
)

func GetCookies() (cookies []*http.Cookie, err error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://www.zhihu.com/people/canglimo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36 Edg/87.0.664.75")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("status code error")
	}

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "d_c0" {
			return resp.Cookies(), nil
		}
	}

	return nil, errors.New("no d_c0 cookie")
}

type encryptReq struct {
	CookieMes string `json:"cookie_mes"`
	ApiPath   string `json:"api_path"`
}

type encryptResp struct {
	XZSE96 string `json:"xzse96"`
}

func GetXZSE96(url, dC0 string) (xzse96 string, err error) {
	path, err := getPath(url)
	if err != nil {
		return "", err
	}

	reqData := encryptReq{
		CookieMes: dC0,
		ApiPath:   path,
	}
	reqBody, err := json.Marshal(reqData)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", config.C.ZhihuEncryptionURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("status code error")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var respData encryptResp
	err = json.Unmarshal(respBody, &respData)
	if err != nil {
		return "", err
	}

	return respData.XZSE96, nil
}

func getPath(path string) (result string, err error) {
	u, err := url.Parse(path)
	if err != nil {
		return "", err
	}

	if u.RawQuery != "" {
		result = fmt.Sprintf("%s?%s", u.Path, u.RawQuery)
	} else {
		result = u.Path
	}

	return result, nil
}

func CookiesToString(cookies []*http.Cookie) string {
	var sb strings.Builder

	for i, cookie := range cookies {
		sb.WriteString(cookie.String())
		if i < len(cookies)-1 {
			sb.WriteString(", ")
		}
	}

	return sb.String()
}
