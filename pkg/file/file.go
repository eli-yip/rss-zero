package file

import (
	"io"
)

type FileIface interface {
	SaveStream(string, io.ReadCloser, int64) error
	GetStream(string) (io.ReadCloser, error)
	AssetsDomain() string
}
