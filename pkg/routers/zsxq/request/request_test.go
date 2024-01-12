package request

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWithLimiterRawData(t *testing.T) {
	cookies := os.Getenv("COOKIES")
	t.Log(cookies)
	rs := NewRequestService(cookies, nil)
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
