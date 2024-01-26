package db

type MockDB struct{}

func (d *MockDB) SavePost(p *Post) error {
	return nil
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
