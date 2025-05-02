package export

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/db"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/render"
)

type Option struct {
	PaperID   string
	StartTime time.Time
	EndTime   time.Time
}

type Exporter interface {
	Export(io.Writer, Option) error
	FileName(Option) string
}

type ExportService struct {
	db db.DB
	mr render.Render
}

func NewExportService(db db.DB, mr render.Render) Exporter {
	return &ExportService{db: db, mr: mr}
}

var ErrTimeOrder = errors.New("start time should be before end time")

func (s *ExportService) Export(w io.Writer, opt Option) (err error) {
	var queryOpt db.Option

	queryOpt.PaperID = opt.PaperID

	if opt.StartTime.After(opt.EndTime) {
		return ErrTimeOrder
	}
	queryOpt.StartTime = opt.StartTime
	queryOpt.EndTime = opt.EndTime

	paper, err := s.db.GetPaper(opt.PaperID)
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(md.H1(paper.Name) + "\n\n"))
	if err != nil {
		return fmt.Errorf("failed to write paper name: %w", err)
	}

	authorName, err := s.db.GetCreatorName(paper.CreatorID)
	if err != nil {
		return err
	}

	_, err = w.Write([]byte("作者：" + authorName + "\n\n"))
	if err != nil {
		return fmt.Errorf("failed to write author name: %w", err)
	}

	var (
		finished  = false
		lastTime time.Time
	)

	for !finished {
		posts, err := s.db.FetchNPost(20, queryOpt)
		if err != nil {
			return err
		}

		if len(posts) == 0 {
			finished = true
			continue
		}
		lastTime = posts[len(posts)-1].CreateAt

		if len(posts) < 20 {
			finished = true
		}

		for i, post := range posts {
			fullText, err := s.mr.Post(&render.Post{
				ID:    post.ID,
				Title: post.Title,
				Time:  post.CreateAt,
				Text:  post.Text,
			})
			if err != nil {
				return fmt.Errorf("failed to render post %s: %w", post.ID, err)
			}

			if _, err := w.Write([]byte(fullText)); err != nil {
				return fmt.Errorf("failed to write post %s: %w", post.ID, err)
			}

			if finished && i == len(posts)-1 {
				break
			}

			if _, err := w.Write([]byte("\n")); err != nil {
				return fmt.Errorf("failed to write post %s: %w", post.ID, err)
			}

			queryOpt.StartTime = lastTime
		}
	}

	return nil
}

func (s *ExportService) FileName(opt Option) (fileName string) {
	fileNameArr := []string{"小报童专栏", opt.PaperID}

	fileNameArr = append(fileNameArr, opt.StartTime.Format("2006-01-02"))
	fileNameArr = append(fileNameArr, opt.EndTime.Format("2006-01-02"))

	return strings.Join(fileNameArr, "-") + ".md"
}
