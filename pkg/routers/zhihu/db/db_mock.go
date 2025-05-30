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

func (d *MockDB) GetSubs() ([]Sub, error) {
	return nil, nil
}

func (d *MockDB) SetStatus(authorID string, subType int, finished bool) error {
	return nil
}

func (d *MockDB) AddSub(authorID string, subType int) error {
	return nil
}

func (d *MockDB) CountAnswer(authorID string) (int, error) {
	return 0, nil
}

func (d *MockDB) CountArticle(authorID string) (int, error) {
	return 0, nil
}

func (d *MockDB) CountPin(authorID string) (int, error) {
	return 0, nil
}

func (d *MockDB) CheckSub(authorID string, subType int) (bool, error) {
	return false, nil
}

func (d *MockDB) FetchNAnswersBeforeTime(n int, t time.Time, userID string) (as []Answer, err error) {
	return nil, nil
}

func (d *MockDB) FetchNArticlesBeforeTime(n int, t time.Time, authorID string) (as []Article, err error) {
	return nil, nil
}

func (d *MockDB) FetchNPinsBeforeTime(n int, t time.Time, authorID string) (ps []Pin, err error) {
	return nil, nil
}
