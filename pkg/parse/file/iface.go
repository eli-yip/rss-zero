package file

type FileIface interface {
	DownloadLink(int) (string, error)
	Save(string, string) error
}
