package common

import (
	"fmt"
	"net/url"
	"path"
)

// https://rss.momoai.me/i/?c=feed&a=add&url_rss=http%3A%2F%2Frsshub%3A1200%2Fzhihu%2Fpeople%2Factivities%2Fshuo-shuo-98-12
func GenerateFreshRSSFeed(freshRSSURL, feedLink string) (feedURL string, err error) {
	parsedURL, err := url.Parse(freshRSSURL)
	if err != nil {
		return "", fmt.Errorf("fail to parse url: %s", freshRSSURL)
	}
	parsedURL.Path = path.Join(parsedURL.Path, "i") + "/"

	params := url.Values{}

	params.Add("a", "add")
	params.Add("c", "feed")
	params.Add("url_rss", feedLink)

	parsedURL.RawQuery = params.Encode()

	return parsedURL.String(), nil
}
