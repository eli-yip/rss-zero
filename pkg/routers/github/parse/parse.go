package parse

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/routers/github/db"
	"github.com/eli-yip/rss-zero/pkg/routers/github/request"
)

type Parser interface {
	ParseAndSaveRelease(subID string, release request.Release) error
}

type ParseService struct {
	db          db.DB
	mdFormatter *md.MarkdownFormatter
}

func NewParseService(db db.DB) Parser {
	return &ParseService{
		db:          db,
		mdFormatter: md.NewMarkdownFormatter(),
	}
}

func (s *ParseService) ParseAndSaveRelease(subID string, release request.Release) (err error) {
	if release.Draft {
		return nil
	}

	formattedBody, err := s.mdFormatter.FormatStr(release.Body)
	if err != nil {
		return fmt.Errorf("failed to format release body: %w", err)
	}

	releaseInDB, err := s.db.GetRelease(release.ID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check release existance: %w", err)
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		if releaseInDB.PublishedAt.After(release.PublishedAt) {
			return nil
		}
	}

	releaseToSave := db.Release{
		ID:          release.ID,
		SubID:       subID,
		URL:         release.HTMLURL,
		Tag:         release.TagName,
		Title:       release.Name,
		Body:        formattedBody,
		PreRelease:  release.Prerelease,
		PublishedAt: release.PublishedAt,
	}

	if err = s.db.SaveRelease(&releaseToSave); err != nil {
		return fmt.Errorf("failed to save release: %w", err)
	}

	return nil
}
