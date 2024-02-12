package encrypt

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// python code:
// import hashlib
// from datetime import datetime, timezone, timedelta
//
// def md5(code: str):
//     return hashlib.md5(code.encode("utf8")).hexdigest()
//
// def get_sign(t):
//     timestamp = str(int(t))
//     return md5(f"dbbc1dd37360b4084c3a69346e0ce2b2.{timestamp}"), timestamp
//
// if __name__ == "__main__":
//     est = timezone(timedelta(hours=+8))
//     dt = datetime(2020, 1, 1, 0, 0, 0, 0, tzinfo=est)
//     print(get_sign(dt.timestamp()))

const key = `dbbc1dd37360b4084c3a69346e0ce2b2.`

// keyword=&limit=20&offset=0&order_by=created_at undefined&tag_name=
// limit=20&offset=0&tag_name=&keyword=&order_by=created_at+undefined

func Sign(t time.Time, u string) (timeStr string, sign string, err error) {
	timestamp := strconv.FormatInt(t.Unix(), 10)
	parsedParams, err := parseQuery(u)
	if err != nil {
		return "", "", err
	}
	hash := md5.Sum([]byte(parsedParams + key + timestamp))
	return timestamp, hex.EncodeToString(hash[:]), nil
}

func parseQuery(u string) (string, error) {
	parseURL, err := url.Parse(u)
	if err != nil {
		return "", err
	}

	values, err := url.ParseQuery(parseURL.RawQuery)
	if err != nil {
		return "", err
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sortedQueryParts []string
	for _, key := range keys {
		for _, value := range values[key] {
			sortedQueryParts = append(sortedQueryParts, fmt.Sprintf("%s=%s", key, url.QueryEscape(value)))
		}
	}
	sortedQuery := strings.Join(sortedQueryParts, "&")
	sortedQuery = strings.ReplaceAll(sortedQuery, "+", " ")

	return sortedQuery, nil
}
