package file

import "io"

type FileIface interface {
	DownloadLink(int) (string, error)
	Save(string, string) error
	Get(string) (io.Reader, error)
}
