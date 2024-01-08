package file

import (
	"io"
	"net/http"
)

type FileIface interface {
	SaveHTTPStream(string, *http.Response) error
	Get(string) (io.ReadCloser, error)
	GetAssetsDomain() string
}
