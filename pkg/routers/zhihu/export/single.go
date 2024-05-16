package export

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

func (s *ExportService) ExportSingle(writer io.Writer, opt Option) (err error) {
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
		return s.exportSingleAnswer(writer, opt)
	case common.TypeZhihuArticle:
		return s.exportSingleArticle(writer, opt)
	case common.TypeZhihuPin:
		return s.exportSinglePin(writer, opt)
	default:
		return errors.New("unknown type")
	}
}

func (s *ExportService) exportSingleAnswer(writer io.Writer, opt Option) (err error) {
	queryOpt := db.FetchAnswerOption{
		FetchOptionBase: db.FetchOptionBase{
			UserID:    opt.AuthorID,
			StartTime: opt.StartTime,
			EndTime:   opt.EndTime,
		},
	}

	var (
		finished bool
		lastTime time.Time
	)

	tempDir, err := os.MkdirTemp("", "rss-zero-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

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

			filename := buildFilename(answer.CreateAt, question.Title)
			if err = os.WriteFile(filepath.Join(tempDir, filename), []byte(fullText), 0755); err != nil {
				return fmt.Errorf("failed to write file %s: %w", filename, err)
			}

			if finished && i == len(answers)-1 {
				break
			}

			queryOpt.StartTime = lastTime
		}
	}

	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	files, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("failed to read temp dir: %w", err)
	}

	for _, file := range files {
		f, err := os.Open(filepath.Join(tempDir, file.Name()))
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", file.Name(), err)
		}
		defer f.Close()

		w, err := zipWriter.Create(file.Name())
		if err != nil {
			return fmt.Errorf("failed to create zip file %s: %w", file.Name(), err)
		}

		if _, err = io.Copy(w, f); err != nil {
			return fmt.Errorf("failed to copy file %s: %w", file.Name(), err)
		}
	}

	return nil
}

func (s *ExportService) exportSingleArticle(writer io.Writer, opt Option) (err error) {
	queryOpt := db.FetchArticleOption{
		FetchOptionBase: db.FetchOptionBase{
			UserID:    opt.AuthorID,
			StartTime: opt.StartTime,
			EndTime:   opt.EndTime,
		},
	}

	var (
		finished bool
		lastTime time.Time
	)

	tempDir, err := os.MkdirTemp("", "rss-zero-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

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

			filename := buildFilename(article.CreateAt, article.Title)
			if err = os.WriteFile(filepath.Join(tempDir, filename), []byte(fullText), 0755); err != nil {
				return fmt.Errorf("failed to write file %s: %w", filename, err)
			}

			if finished && i == len(articles)-1 {
				break
			}

			queryOpt.StartTime = lastTime
		}
	}

	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	files, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("failed to read temp dir: %w", err)
	}

	for _, file := range files {
		f, err := os.Open(filepath.Join(tempDir, file.Name()))
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", file.Name(), err)
		}
		defer f.Close()

		w, err := zipWriter.Create(file.Name())
		if err != nil {
			return fmt.Errorf("failed to create zip file %s: %w", file.Name(), err)
		}

		if _, err = io.Copy(w, f); err != nil {
			return fmt.Errorf("failed to copy file %s: %w", file.Name(), err)
		}
	}

	return nil
}

func (s *ExportService) exportSinglePin(writer io.Writer, opt Option) (err error) {
	queryOpt := db.FetchPinOption{
		FetchOptionBase: db.FetchOptionBase{
			UserID:    opt.AuthorID,
			StartTime: opt.StartTime,
			EndTime:   opt.EndTime,
		},
	}

	var (
		finished bool
		lastTime time.Time
	)

	tempDir, err := os.MkdirTemp("", "rss-zero-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

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
				BaseContent: render.BaseContent{
					ID:       pin.ID,
					CreateAt: pin.CreateAt,
					Text:     pin.Text,
				},
			})
			if err != nil {
				return err
			}

			filename := buildFilename(pin.CreateAt, pin.Title)
			if err = os.WriteFile(filepath.Join(tempDir, filename), []byte(fullText), 0755); err != nil {
				return fmt.Errorf("failed to write file %s: %w", filename, err)
			}

			if finished && i == len(pins)-1 {
				break
			}

			queryOpt.StartTime = lastTime
		}
	}

	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	files, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("failed to read temp dir: %w", err)
	}

	for _, file := range files {
		f, err := os.Open(filepath.Join(tempDir, file.Name()))
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", file.Name(), err)
		}
		defer f.Close()

		w, err := zipWriter.Create(file.Name())
		if err != nil {
			return fmt.Errorf("failed to create zip file %s: %w", file.Name(), err)
		}

		if _, err = io.Copy(w, f); err != nil {
			return fmt.Errorf("failed to copy file %s: %w", file.Name(), err)
		}
	}

	return nil
}

func buildFilename(t time.Time, title string) (filename string) {
	return escapeFilename(fmt.Sprintf("%s-%s.md", t.Format("2006-01-02"), title))
}

func (s ExportService) FilenameSingle(opt Option) (filename string, err error) {
	fileNameArr := []string{"知乎合集-单文件版"}

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
	// HACK: -1 day to make the end time inclusive: https://git.darkeli.com/yezi/rss-zero/issues/55
	fileNameArr = append(fileNameArr, opt.EndTime.Add(-1*time.Hour*24).Format("2006-01-02"))

	return fmt.Sprintf("%s.zip", strings.Join(fileNameArr, "-")), nil
}

func escapeFilename(filename string) string {
	escapeChars := map[rune]string{
		' ':  "\\ ",
		'!':  "\\!",
		'$':  "\\$",
		'&':  "\\&",
		'\'': "\\'",
		'(':  "\\(",
		')':  "\\)",
		'*':  "\\*",
		';':  "\\;",
		'<':  "\\<",
		'=':  "\\=",
		'>':  "\\>",
		'?':  "\\?",
		'[':  "\\[",
		'\\': "\\\\",
		']':  "\\]",
		'^':  "\\^",
		'`':  "\\`",
		'{':  "\\{",
		'|':  "\\|",
		'}':  "\\}",
		'~':  "\\~",
		'/':  "\\/",
	}

	var escaped strings.Builder

	for i := 0; i < len(filename); {
		r, size := utf8.DecodeRuneInString(filename[i:])
		if escapeSeq, ok := escapeChars[r]; ok {
			escaped.WriteString(escapeSeq)
		} else {
			escaped.WriteRune(r)
		}
		i += size
	}

	return escaped.String()
}
