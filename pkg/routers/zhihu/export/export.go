package export

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

type Option struct {
	AuthorID  *string
	Type      *int
	StartTime time.Time
	EndTime   time.Time
}

type Exporter interface {
	Export(io.Writer, Option) error
	ExportSingle(io.Writer, Option) error
	Filename(Option) (string, error)
	FilenameSingle(Option) (string, error)
}

type ExportService struct {
	db db.DB
	mr render.FullTextRenderIface
}

func NewExportService(db db.DB, mr render.FullTextRenderIface) Exporter {
	return &ExportService{db: db, mr: mr}
}

var (
	ErrNoAuthor  = errors.New("no author found")
	ErrTimeOrder = errors.New("start time should be before end time")
)

func (s *ExportService) Export(writer io.Writer, opt Option) (err error) {
	if opt.Type == nil {
		return errors.New("type is required")
	}

	if opt.AuthorID == nil {
		return ErrNoAuthor
	}

	if opt.StartTime.After(opt.EndTime) {
		return ErrTimeOrder
	}

	switch *opt.Type {
	case common.TypeZhihuAnswer:
		return s.exportAnswer(writer, opt)
	case common.TypeZhihuArticle:
		return s.exportArticle(writer, opt)
	case common.TypeZhihuPin:
		return s.exportPin(writer, opt)
	default:
		return errors.New("unknown type")
	}
}

func (s *ExportService) exportAnswer(writer io.Writer, opt Option) (err error) {
	var queryOpt db.FetchAnswerOption
	queryOpt.UserID = opt.AuthorID
	queryOpt.StartTime = opt.StartTime
	queryOpt.EndTime = opt.EndTime

	var (
		finished bool
		lastTime time.Time
	)

	for !finished {
		answers, err := s.db.FetchNAnswer(20, queryOpt)
		if err != nil {
			return err
		}

		if len(answers) == 0 {
			finished = true
			continue
		}
		lastTime = answers[len(answers)-1].CreateAt

		if len(answers) < 20 {
			finished = true
		}

		for i, answer := range answers {
			question, err := s.db.GetQuestion(answer.QuestionID)
			if err != nil {
				return err
			}

			fullText, err := s.mr.Answer(&render.Answer{
				Question: render.BaseContent{
					ID:       question.ID,
					CreateAt: question.CreateAt,
					Text:     question.Title,
				},
				Answer: render.BaseContent{
					ID:       answer.ID,
					CreateAt: answer.CreateAt,
					Text:     answer.Text,
				},
			})
			if err != nil {
				return err
			}

			if _, err := writer.Write([]byte(fullText)); err != nil {
				return err
			}

			if finished && i == len(answers)-1 {
				break
			}

			if _, err := writer.Write([]byte("\n")); err != nil {
				return err
			}

			queryOpt.StartTime = lastTime
		}
	}

	return nil
}

func (s *ExportService) exportArticle(writer io.Writer, opt Option) (err error) {
	var queryOpt db.FetchArticleOption
	queryOpt.UserID = opt.AuthorID
	queryOpt.StartTime = opt.StartTime
	queryOpt.EndTime = opt.EndTime

	var (
		finished bool
		lastTime time.Time
	)

	for !finished {
		articles, err := s.db.FetchNArticle(20, queryOpt)
		if err != nil {
			return err
		}

		if len(articles) == 0 {
			finished = true
			continue
		}
		lastTime = articles[len(articles)-1].CreateAt

		if len(articles) < 20 {
			finished = true
		}

		for i, article := range articles {
			fullText, err := s.mr.Article(&render.Article{
				Title: article.Title,
				BaseContent: render.BaseContent{
					ID:       article.ID,
					CreateAt: article.CreateAt,
					Text:     article.Text},
			})
			if err != nil {
				return err
			}

			if _, err := writer.Write([]byte(fullText)); err != nil {
				return err
			}

			if finished && i == len(articles)-1 {
				break
			}

			if _, err := writer.Write([]byte("\n")); err != nil {
				return err
			}

			queryOpt.StartTime = lastTime
		}
	}

	return nil
}

func (s *ExportService) exportPin(writer io.Writer, opt Option) (err error) {
	var queryOpt db.FetchPinOption
	queryOpt.UserID = opt.AuthorID
	queryOpt.StartTime = opt.StartTime
	queryOpt.EndTime = opt.EndTime

	var (
		finished bool
		lastTime time.Time
	)

	for !finished {
		pins, err := s.db.FetchNPin(20, queryOpt)
		if err != nil {
			return err
		}

		if len(pins) == 0 {
			finished = true
			continue
		}
		lastTime = pins[len(pins)-1].CreateAt

		if len(pins) < 20 {
			finished = true
		}

		for i, pin := range pins {
			fullText, err := s.mr.Pin(&render.Pin{
				Title: pin.Title,
				BaseContent: render.BaseContent{
					ID:       pin.ID,
					CreateAt: pin.CreateAt,
					Text:     pin.Text,
				},
			})
			if err != nil {
				return err
			}

			if _, err := writer.Write([]byte(fullText)); err != nil {
				return err
			}

			if finished && i == len(pins)-1 {
				break
			}

			if _, err := writer.Write([]byte("\n")); err != nil {
				return err
			}

			queryOpt.StartTime = lastTime
		}
	}

	return nil
}

func (s ExportService) Filename(opt Option) (filename string, err error) {
	fileNameArr := []string{"知乎合集"}

	switch *opt.Type {
	case common.TypeZhihuAnswer:
		fileNameArr = append(fileNameArr, "回答")
	case common.TypeZhihuArticle:
		fileNameArr = append(fileNameArr, "文章")
	case common.TypeZhihuPin:
		fileNameArr = append(fileNameArr, "想法")
	}

	authorName, err := s.db.GetAuthorName(*opt.AuthorID)
	if err != nil {
		return "", fmt.Errorf("failed to get author name: %w", err)
	}
	if strings.Contains(authorName, "-") {
		authorName = strings.ReplaceAll(authorName, "-", "_")
	}
	fileNameArr = append(fileNameArr, authorName)

	fileNameArr = append(fileNameArr, opt.StartTime.Format("2006-01-02"))
	// HACK: -1 day to make the end time inclusive: https://git.momoai.me/yezi/rss-zero/issues/55
	fileNameArr = append(fileNameArr, opt.EndTime.Add(-1*time.Hour*24).Format("2006-01-02"))

	return fmt.Sprintf("%s.md", strings.Join(fileNameArr, "-")), nil
}
