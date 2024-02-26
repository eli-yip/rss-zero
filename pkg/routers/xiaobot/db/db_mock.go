package db

import "time"

type DBMock struct{}

func NewDBMock() DB { return &DBMock{} }

func (d *DBMock) SavePost(post *Post) (err error) { return nil }

func (d *DBMock) SavePaper(paper *Paper) (err error) { return nil }

func (d *DBMock) GetLatestTime(paperID string) (t time.Time, err error) { return time.Time{}, nil }

func (d *DBMock) GetPapers() ([]Paper, error) { return nil, nil }

func (d *DBMock) GetPaper(id string) (*Paper, error) { return nil, nil }

func (d *DBMock) CheckPaper(id string) (bool, error) { return false, nil }

func (d *DBMock) GetLatestNPost(paperID string, n int) ([]Post, error) { return nil, nil }

func (d *DBMock) SaveCreator(creator *Creator) (err error) { return nil }

func (d *DBMock) GetCreatorName(id string) (string, error) { return "", nil }

func (d *DBMock) FetchNPost(n int, opt Option) (ps []Post, err error) { return nil, nil }

func (d *DBMock) FetchNPostBeforeTime(n int, paperID string, t time.Time) ([]Post, error) {
	return nil, nil
}
