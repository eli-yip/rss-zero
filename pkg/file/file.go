package file

import (
	"io"
)

type FileIface interface {
	SaveHTTPStream(string, io.ReadCloser) error
	Get(string) (io.ReadCloser, error)
	GetAssetsDomain() string
}
