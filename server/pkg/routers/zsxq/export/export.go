package export

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"gorm.io/gorm"
)

type Option struct {
	GroupID    int
	Type       *string
	Digested   *bool
	AuthorName *string
	StartTime  time.Time
	EndTime    time.Time
}

type Exporter interface {
	Export(io.Writer, Option) error
	FileName(opt Option) string
}

type ExportService struct {
	db db.DB
	mr render.FullTextRenderer
}

func NewExportService(db db.DB, mr render.FullTextRenderer) Exporter {
	return &ExportService{db: db, mr: mr}
}

var (
	ErrNoAuthor  = errors.New("no author found")
	ErrTimeOrder = errors.New("start time should be before end time")
)

func (s *ExportService) Export(writer io.Writer, opt Option) (err error) {
	var queryOpt db.Options

	queryOpt.GroupID = opt.GroupID

	if opt.Type != nil {
		queryOpt.Type = opt.Type
	}

	if opt.Digested != nil {
		queryOpt.Digested = opt.Digested
	}

	if opt.AuthorName != nil {
		aid, err := s.db.GetAuthorID(*opt.AuthorName)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNoAuthor
			}
			return err
		}
		queryOpt.Aid = &aid
	}

	if opt.StartTime.After(opt.EndTime) {
		return ErrTimeOrder
	}
	queryOpt.StartTime = opt.StartTime
	queryOpt.EndTime = opt.EndTime

	var (
		finished = false
		lastTime time.Time
	)

	for !finished {
		topics, err := s.db.FetchNTopics(20, queryOpt)
		if err != nil {
			return err
		}

		if len(topics) == 0 {
			finished = true
			continue
		}
		lastTime = topics[len(topics)-1].Time

		if len(topics) < 20 {
			finished = true
		}

		for i, topic := range topics {
			fullText, err := s.mr.FullText(
				&render.Topic{
					ID:       topic.ID,
					GroupID:  topic.GroupID,
					Title:    topic.Title,
					Type:     topic.Type,
					Digested: topic.Digested,
					Time:     topic.Time,
					Text:     topic.Text,
				},
			)
			if err != nil {
				return err
			}

			if _, err := writer.Write([]byte(fullText)); err != nil {
				return err
			}

			if finished && i == len(topics)-1 {
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

func (s *ExportService) FileName(opt Option) string {
	fileNameArr := []string{"知识星球合集", strconv.Itoa(opt.GroupID)}

	if opt.Type != nil {
		fileNameArr = append(fileNameArr, *opt.Type)
	}

	if opt.Digested != nil {
		if *opt.Digested {
			fileNameArr = append(fileNameArr, "digest")
		} else {
			fileNameArr = append(fileNameArr, "all")
		}
	}

	if opt.AuthorName != nil {
		fileNameArr = append(fileNameArr, *opt.AuthorName)
	}

	fileNameArr = append(fileNameArr, opt.StartTime.Format("2006-01-02"))
	// HACK: -1 day to make the end time inclusive: https://git.momoai.me/yezi/rss-zero/issues/55
	fileNameArr = append(fileNameArr, opt.EndTime.Add(-1*time.Hour*24).Format("2006-01-02"))

	return strings.Join(fileNameArr, "-") + ".md"
}
