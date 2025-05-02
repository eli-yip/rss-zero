package db

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
)

type DBAnswer interface {
	// Save answer info to zhihu_answer table
	SaveAnswer(a *Answer) error
	GetLatestNAnswer(n int, userID string) ([]Answer, error)
	// FetchNAnswers get n answers from zhihu_answer table,
	// then return the answers for text generating.
	FetchNAnswer(int, FetchAnswerOption) ([]Answer, error)
	FetchAnswer(author string, limit, offset int) ([]Answer, error)
	FetchAnswerByIDs(ids []int) (map[int]Answer, error)
	FetchAnswerWithDateRange(author string, limit, offset, order int, startTime, endTime time.Time) ([]Answer, error)
	// UpdateAnswerStatus update answer status in zhihu_answer table
	UpdateAnswerStatus(id int, status int) error
	// GetLatestAnswerTime get the latest answer time from zhihu_answer table
	GetLatestAnswerTime(userID string) (time.Time, error)
	// GetAnswerAfter get answer after t from zhihu_answer table
	GetAnswerAfter(userID string, t time.Time) ([]Answer, error)
	// GetAnswer get answer info from zhihu_answer table
	GetAnswer(id int) (*Answer, error)
	CountAnswer(userID string) (int, error)
	CountAnswerWithDateRange(userID string, startTime, endTime time.Time) (int, error)
	FetchNAnswersBeforeTime(n int, t time.Time, userID string) ([]Answer, error)
	// RandomSelect select n random answers from zhihu_answer table
	//
	// Answers are created after 2023-01-01, and the word count is between 300 and 1200.
	RandomSelect(n int, userID string) ([]Answer, error)
	SelectByID(ids []int) ([]Answer, error)
}

type FetchAnswerOption struct {
	FetchOptionBase

	Text   *string
	Status *int
}

// "content": "\u003cp data-pid=\"TmR0DPFm\"\u003e 有很多逻辑，其实都是词汇定义出了问题。\u003c/p\u003e\u003cp data-pid=\"SzvAo4_H\"\u003e 把被动的，掩饰成主动的，然后再包装成所谓的竞争优势。\u003c/p\u003e\u003cp data-pid=\"5j3nbEqH\"\u003e 这时候，就扭曲了基本逻辑，从而导致了应然和实然的冲突。\u003c/p\u003e\u003cp data-pid=\"X1J0RGRZ\"\u003e 以题目中“保守”的语境而言。\u003c/p\u003e\u003cp data-pid=\"Gd3KsLn1\"\u003e 如果一位女性，魅力突出、花容月貌、才华横溢，而又洁身自好，那么，这是一种褒义的“保守”。\u003c/p\u003e\u003cp data-pid=\"QqmczkmK\"\u003e 注意，这种保守，是这位女性自己选择的。\u003c/p\u003e\u003cp data-pid=\"_qEBUjq9\"\u003e 另外一位女性，出门回头率零，邋邋遢遢从不收拾自己，她没有异性交往经历，这也是一种“保守”。\u003c/p\u003e\u003cp data-pid=\"_LFutO-F\"\u003e 但，第二种保守，是被动的，至少在她学会捯饬自己之前，没有异性愿意跟她交往，那么她没有“不保守”的选择。\u003c/p\u003e\u003cp data-pid=\"GhuwIZXx\"\u003e 这时候，这个“保守”就不是褒义的，而是假象。\u003c/p\u003e\u003cp data-pid=\"pY6psRn_\"\u003e 类似的案例很多，更常见的，我以前说过，“善良”。\u003c/p\u003e\u003cp data-pid=\"CANVglqI\"\u003e 胸怀利刃而不害人，是善良。\u003c/p\u003e\u003cp data-pid=\"YEw371Vc\"\u003e 胸无利刃，而四处说自己不害人，是被迫善良，不是真的善良。\u003c/p\u003e\u003cp data-pid=\"arJhegAK\"\u003e 另外，用“保守”去褒义化女性，污名化有异性亲密经历的女性，这种思维本身也物化了女性，并且暗示了女性的亲密经历是吃亏的。\u003c/p\u003e\u003cp data-pid=\"bq3Ej6XK\"\u003e 这其实会带来非常多的问题。\u003c/p\u003e\u003cp data-pid=\"nXadeqX9\"\u003e 比如女性被某个长期非常痛苦的亲密关系绑架，而由于认为自己已经付出了贞洁（因而沉没成本太高），而不敢分手。\u003c/p\u003e\u003cp data-pid=\"-R5dQS7U\"\u003e 很多事情，还是要分事情，辩证的看。\u003c/p\u003e\u003cp data-pid=\"IZfWnkEW\"\u003e 在很多很多年前，有种伦理，女性未出嫁不小心碰到男性，就自杀，然后立个牌坊给家里人免税。\u003c/p\u003e\u003cp data-pid=\"YEssMHOc\"\u003e 那个年代，没有拿女性当人看的，是吃人的年代。\u003c/p\u003e\u003cp data-pid=\"Pe3lcCr5\"\u003e 在现代，类似的吃人逻辑，应该消失了。\u003c/p\u003e\u003cp data-pid=\"rq5i9sVN\"\u003e 不要过着现代化生活，而脑子还在古代。\u003c/p\u003e\u003cp data-pid=\"WlD9gx6L\"\u003e 供参考。\u003c/p\u003e\u003cp\u003e\u003c/p\u003e",
// "created_time": 1705985791,
// "id": 3372966744,
type Answer struct {
	ID         int       `gorm:"column:id;type:int;primary_key"`
	QuestionID int       `gorm:"column:question_id;type:int"`
	AuthorID   string    `gorm:"column:author_id;type:text"`
	CreateAt   time.Time `gorm:"column:create_at;type:timestamptz"`
	UpdateAt   time.Time `gorm:"column:update_at;type:timestamptz"`
	Text       string    `gorm:"column:text;type:text"`
	// NOTE: raw can be standard apiModel.Answer,
	// or raw from zhihu api,
	// it depends on how parseAnswer func is used.
	// If parseAnswer func is used to parse the standard apiModel.Answer from answerList,
	// then raw is the standard apiModel.Answer.
	// If parseAnswer func is used to parse the raw from zhihu api,
	// then raw is the raw from zhihu api.
	Raw    []byte `gorm:"column:raw;type:bytea"`
	Status int    `gorm:"column:status;type:int"`

	WordCount int `gorm:"column:word_count;type:int"`
}

const (
	AnswerStatusUncompleted = iota
	AnswerStatusCompleted
	AnswerStatusUnreachable
)

func (a *Answer) TableName() string { return "zhihu_answer" }

//	"question": {
//	  "created": 1705768292,
//	  "id": 640511134,
//	  "title": "为什么那么多人就是不愿意承认女生保守是一个极大的竞争优势？"
//	}
type Question struct {
	ID       int       `gorm:"column:id;type:int;primary_key"`
	CreateAt time.Time `gorm:"column:create_at;type:timestamptz"`
	Title    string    `gorm:"column:title;type:text"`
}

func (q *Question) TableName() string { return "zhihu_question" }

func (d *DBService) SaveAnswer(a *Answer) error { return d.Save(a).Error }

func (d *DBService) GetLatestNAnswer(n int, userID string) ([]Answer, error) {
	as := make([]Answer, 0, n)
	if err := d.Where("author_id = ?", userID).Order("create_at desc").Limit(n).Find(&as).Error; err != nil {
		return nil, err
	}
	return as, nil
}

func (d *DBService) FetchNAnswersBeforeTime(n int, t time.Time, userID string) (as []Answer, err error) {
	err = d.Where("author_id = ? and create_at < ?", userID, t).Order("create_at desc").Limit(n).Find(&as).Error
	return as, err
}

func (d *DBService) FetchAnswerByIDs(ids []int) (map[int]Answer, error) {
	answers := make([]Answer, 0, len(ids))
	if err := d.Where("id in ?", ids).Find(&answers).Error; err != nil {
		return nil, fmt.Errorf("failed to get answers by ids: %w", err)
	}

	result := make(map[int]Answer, len(answers))
	for answer := range slices.Values(answers) {
		result[answer.ID] = answer
	}

	return result, nil
}

func (d *DBService) CountAnswer(userID string) (int, error) {
	var count int64
	if err := d.Model(&Answer{}).Where("author_id = ?", userID).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func (d *DBService) CountAnswerWithDateRange(userID string, startTime, endTime time.Time) (int, error) {
	var count int64
	if err := d.Model(&Answer{}).Where("author_id = ?", userID).Where("create_at >= ?", startTime).Where("create_at <= ?", endTime).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func (d *DBService) FetchNAnswer(n int, opts FetchAnswerOption) (as []Answer, err error) {
	as = make([]Answer, 0, n)

	query := d.Limit(n)

	if opts.UserID != nil {
		query = query.Where("author_id = ?", *opts.UserID)
	}

	if !opts.StartTime.IsZero() {
		query = query.Where("create_at >= ?", opts.StartTime)
	}

	if !opts.EndTime.IsZero() {
		query = query.Where("create_at <= ?", opts.EndTime)
	}

	if opts.Text != nil {
		query = query.Where("text = ?", *opts.Text)
	}

	if opts.Status != nil {
		query = query.Where("status = ?", *opts.Status)
	}

	if err := query.Order("create_at asc").Find(&as).Error; err != nil {
		return nil, err
	}

	return as, nil
}

func (d *DBService) FetchAnswer(author string, limit, offset int) ([]Answer, error) {
	as := make([]Answer, 0, limit)
	if err := d.Where("author_id = ?", author).Order("create_at desc").Limit(limit).Offset(offset).Find(&as).Error; err != nil {
		return nil, err
	}
	return as, nil
}

func (d *DBService) FetchAnswerWithDateRange(author string, limit, offset, order int, startTime, endTime time.Time) ([]Answer, error) {
	as := make([]Answer, 0, limit)
	stmt := d.Where("author_id = ?", author).Where("create_at >= ?", startTime).Where("create_at < ?", endTime).Limit(limit).Offset(offset)
	if order == 0 {
		stmt = stmt.Order("create_at desc")
	} else {
		stmt = stmt.Order("create_at asc")
	}
	if err := stmt.Find(&as).Error; err != nil {
		return nil, err
	}
	return as, nil
}

func (d *DBService) GetLatestAnswerTime(userID string) (time.Time, error) {
	var t time.Time
	if err := d.Model(&Answer{}).Where("author_id = ?", userID).Order("create_at desc").Limit(1).Pluck("create_at", &t).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return t, nil
}

func (d *DBService) GetAnswerAfter(userID string, t time.Time) ([]Answer, error) {
	as := make([]Answer, 0)
	if err := d.Where("author_id = ? and create_at > ?", userID, t).Order("create_at desc").Find(&as).Error; err != nil {
		return nil, err
	}
	return as, nil
}

func (d *DBService) GetAnswer(id int) (*Answer, error) {
	var a Answer
	if err := d.Where("id = ?", id).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (d *DBService) UpdateAnswerStatus(id int, status int) error {
	return d.Model(&Answer{}).Where("id = ?", id).Update("status", status).Error
}

func (d *DBService) RandomSelect(n int, userID string) (answers []Answer, err error) {
	/**
	 * Note: The following code is not efficient, but it is simple:
	 * 1. Get all answer ids of the user.
	 * 2. Shuffle the answer ids.
	 * 3. Select the first n answer ids.
	 */

	answers = make([]Answer, 0, n)

	answerIDs := make([]int, 0, n)
	date := time.Date(2023, 1, 1, 0, 0, 0, 0, config.C.BJT)
	d.Model(&Answer{}).Where("author_id = ?", userID).Where("create_at >= ?", date).Where("word_count between ? and ?", 300, 1200).Pluck("id", &answerIDs)

	if len(answerIDs) <= n {
		if err = d.Where("id in ?", answerIDs).Find(&answers).Error; err != nil {
			return nil, fmt.Errorf("failed to get answers: %w", err)
		}
		return answers, nil
	}

	// shuffle answerIDs
	rand.Shuffle(len(answerIDs), func(i, j int) {
		answerIDs[i], answerIDs[j] = answerIDs[j], answerIDs[i]
	})

	answerIDs = answerIDs[:n]

	if err := d.Where("id in ?", answerIDs).Find(&answers).Error; err != nil {
		return nil, fmt.Errorf("failed to get answers: %w", err)
	}

	return answers, nil
}

func (d *DBService) SelectByID(ids []int) (answers []Answer, err error) {
	answers = make([]Answer, 0, len(ids))
	if err := d.Where("id in ?", ids).Find(&answers).Error; err != nil {
		return nil, fmt.Errorf("failed to get answers: %w", err)
	}
	return answers, nil
}

type DBQuestion interface {
	// GetQuestion get question info from zhihu_question table
	GetQuestion(id int) (*Question, error)
	// GetQuestions get questions info from zhihu_question table
	GetQuestions(ids []int) ([]Question, error)
	// Save question info to zhihu_question table
	SaveQuestion(q *Question) error
}

func (d *DBService) SaveQuestion(q *Question) error { return d.Save(q).Error }

func (d *DBService) GetQuestion(id int) (*Question, error) {
	var q Question
	if err := d.Where("id = ?", id).First(&q).Error; err != nil {
		return nil, err
	}
	return &q, nil
}

func (d *DBService) GetQuestions(ids []int) ([]Question, error) {
	qs := make([]Question, 0, len(ids))
	if err := d.Where("id in ?", ids).Find(&qs).Error; err != nil {
		return nil, fmt.Errorf("failed to get questions: %w", err)
	}
	return qs, nil
}
