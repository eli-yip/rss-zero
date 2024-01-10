package file

import (
	"io"
)

// FileIface save(both stream and bytes) and get file(stream and assets domain)
type FileIface interface {
	// Save stream to object storage
	SaveStream(string, io.ReadCloser) error
	// Get stream from object storage with object key
	Get(string) (io.ReadCloser, error)
	// Get assets domain from implementation
	GetAssetsDomain() string
}
