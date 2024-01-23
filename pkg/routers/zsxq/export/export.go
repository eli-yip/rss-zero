package export

import (
	"errors"
	"io"
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"gorm.io/gorm"
)

type Options struct {
	GroupID    int
	Type       *string
	Digested   *bool
	AuthorName *string
	StartTime  time.Time
	EndTime    time.Time
}

type Exporter interface {
	Export(io.Writer, Options) error
}

type ExportService struct {
	db db.DataBaseIface
	mr render.MarkdownRenderer
}

func NewExportService(db db.DataBaseIface, mr render.MarkdownRenderer) *ExportService {
	return &ExportService{db: db, mr: mr}
}

var (
	ErrNoAuthor  = errors.New("no author found")
	ErrTimeOrder = errors.New("start time should be before end time")
)

func (s *ExportService) Export(writer io.Writer, opt Options) error {
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
			if err == gorm.ErrRecordNotFound {
				return ErrNoAuthor
			}
			return err
		}
		queryOpt.Aid = &aid
	}

	if opt.StartTime.IsZero() && opt.EndTime.IsZero() {
		if opt.StartTime.After(opt.EndTime) {
			return ErrTimeOrder
		}
	}

	if !opt.StartTime.IsZero() {
		queryOpt.StartTime = opt.StartTime
	}

	if !opt.EndTime.IsZero() {
		queryOpt.EndTime = opt.EndTime
	}

	var (
		finished bool = false
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
			fullText, err := s.mr.ToFullText(
				&render.Topic{
					ID:        topic.ID,
					Title:     topic.Title,
					Type:      topic.Type,
					Digested:  topic.Digested,
					Time:      topic.Time,
					ShareLink: topic.ShareLink,
					Text:      topic.Text,
				},
			)
			if err != nil {
				return err
			}

			if _, err := writer.Write(fullText); err != nil {
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
