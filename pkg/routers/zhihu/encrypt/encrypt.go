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

	"github.com/PuerkitoBio/goquery"
	"github.com/eli-yip/rss-zero/config"
	"go.uber.org/zap"
)

var errParseHTML = errors.New("failed to parse html")

// GetCookies get cookies from zhihu.
//
// if any bad request or too many requests, return error.
//
// if there is no d_c0 cookie, return error.
func GetCookies(logger *zap.Logger) (cookies []*http.Cookie, err error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://www.zhihu.com/people/canglimo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36 Edg/87.0.664.75")
	req.Header.Set("Accept", "*/*")

	count := 3
	for count <= 0 {
		count--

		resp, err := client.Do(req)
		if err != nil {
			logger.Error("Fail to do request", zap.Error(err))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("response status code error: %d", resp.StatusCode)
			logger.Error("Bad response status", zap.Error(err))
			continue
		}
		logger.Info("Got zhihu response from https://www.zhihu.com/people/canglimo, try to get d_c0 cookie")

		for _, cookie := range resp.Cookies() {
			if cookie.Name == "d_c0" {
				return resp.Cookies(), nil
			}
		}

		if err = checkTooManyRequest(resp.Body); err != nil {
			logger.Error("Too many request", zap.Error(err))
		}
		logger.Info("Found no d_c0 cookie, try again")
	}

	if err == nil {
		return nil, errors.New("no d_c0 cookie")
	}
	return nil, err
}

func checkTooManyRequest(body io.Reader) (err error) {
	doc, err := goquery.NewDocumentFromReader(body)
	if err != nil {
		return errors.Join(errParseHTML, err)
	}

	found := false
	_ = doc.Find("head title").Each(func(i int, s *goquery.Selection) {
		if strings.TrimSpace(s.Text()) == "安全验证 - 知乎" && s.AttrOr("data-rh", "") == "true" {
			found = true
		}
	})

	if found {
		return errors.New("too many requests")
	}
	return nil
}

type (
	// this struct is used to send request to zhihu encryption api, see in https://gitea.momoai.me/yezi/zhihu-encrypt
	encryptReq struct {
		CookieMes string `json:"cookie_mes"`
		ApiPath   string `json:"api_path"`
	}

	// this struct is used to parse response from zhihu encryption api, see in https://gitea.momoai.me/yezi/zhihu-encrypt
	encryptResp struct {
		XZSE96 string `json:"xzse96"`
	}
)

// GetXZSE96 get xzse96 from zhihu encryption api.
func GetXZSE96(url, dC0 string) (xzse96 string, err error) {
	pathAndQuery, err := getPathAndQuery(url)
	if err != nil {
		return "", err
	}

	reqBody, err := json.Marshal(encryptReq{
		CookieMes: dC0,
		ApiPath:   pathAndQuery,
	})
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
		return "", fmt.Errorf("status code error: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var respData encryptResp
	if err = json.Unmarshal(respBody, &respData); err != nil {
		return "", err
	}

	return respData.XZSE96, nil
}

// getPathAndQuery get url path and query string.
func getPathAndQuery(path string) (result string, err error) {
	u, err := url.Parse(path)
	if err != nil {
		return "", err
	}

	if u.RawQuery != "" {
		return fmt.Sprintf("%s?%s", u.Path, u.RawQuery), nil
	}
	return u.Path, nil
}

// CookiesToString convert cookies to string. Cookies are separated by comma and space.
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
