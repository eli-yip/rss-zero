package db

import "time"

type MockDB struct{}

func (d *MockDB) SaveArticle(p *Article) error {
	return nil
}

func (d *MockDB) GetLatestNAnswer(n int, id string) ([]Answer, error) {
	return nil, nil
}

func (d *MockDB) GetLatestNArticle(n int, id string) ([]Article, error) {
	return nil, nil
}

func (d *MockDB) GetLatestNPin(n int, id string) ([]Pin, error) {
	return nil, nil
}

func (d *MockDB) SaveQuestion(q *Question) error {
	return nil
}

func (d *MockDB) SaveAuthor(a *Author) error {
	return nil
}

func (d *MockDB) SaveObjectInfo(o *Object) error {
	return nil
}

func (d *MockDB) GetObjectInfo(id int) (o *Object, err error) {
	return nil, nil
}

func (d *MockDB) SaveAnswer(a *Answer) error {
	return nil
}

func (d *MockDB) FetchNAnswer(n int, opts FetchAnswerOption) (as []Answer, err error) {
	return nil, nil
}

func (d *MockDB) UpdateAnswerStatus(id int, status int) error {
	return nil
}

func (d *MockDB) SavePin(p *Pin) error {
	return nil
}

func (d *MockDB) GetAuthorName(string) (string, error) {
	return "", nil
}

func (d *MockDB) GetLatestAnswerTime(string) (time.Time, error) {
	return time.Time{}, nil
}

func (d *MockDB) GetLatestArticleTime(string) (time.Time, error) {
	return time.Time{}, nil
}

func (d *MockDB) GetLatestPinTime(string) (time.Time, error) {
	return time.Time{}, nil
}

func (d *MockDB) FetchNArticle(n int, opts FetchArticleOption) (as []Article, err error) {
	return nil, nil
}

func (d *MockDB) FetchNPin(n int, opts FetchPinOption) (ps []Pin, err error) {
	return nil, nil
}

func (d *MockDB) GetQuestion(id int) (*Question, error) {
	return nil, nil
}
