package parse

import (
	"errors"
	"fmt"

	"github.com/pemistahl/lingua-go"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/ai"
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/routers/github/db"
	"github.com/eli-yip/rss-zero/pkg/routers/github/request"
)

type Parser interface {
	ParseAndSaveRelease(repoID string, release request.Release) error
	detectLanguage(text string) (lingua.Language, bool)
}

type ParseService struct {
	db               db.DB
	ai               ai.AI
	mdFormatter      *md.MarkdownFormatter
	languageDetector lingua.LanguageDetector
}

func NewParseService(db db.DB, ai ai.AI) Parser {
	languages := []lingua.Language{
		lingua.English,
		lingua.Chinese,
	}
	detector := lingua.NewLanguageDetectorBuilder().
		FromLanguages(languages...).Build()

	return &ParseService{
		db:               db,
		ai:               ai,
		mdFormatter:      md.NewMarkdownFormatter(),
		languageDetector: detector,
	}
}

func (s *ParseService) ParseAndSaveRelease(repoID string, release request.Release) (err error) {
	if release.Draft {
		return nil
	}

	releaseInDB, err := s.db.GetRelease(release.ID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check release existance: %w", err)
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		if !releaseInDB.PublishedAt.Before(release.PublishedAt) {
			return nil
		}
	}

	rawBody := release.Body
	language, exists := s.detectLanguage(release.Body)
	if exists {
		if len(release.Body) > 200 && language == lingua.English {
			translatedBody, err := s.ai.TranslateToZh(release.Body)
			if err != nil {
				return fmt.Errorf("failed to translate release body: %w", err)
			}
			release.Body = translatedBody
		}
	}

	formattedBody, err := s.mdFormatter.FormatStr(release.Body)
	if err != nil {
		return fmt.Errorf("failed to format release body: %w", err)
	}

	releaseToSave := db.Release{
		ID:     release.ID,
		RepoID: repoID,
		URL:    release.HTMLURL,
		Tag:    release.TagName,
		Title: func() string {
			if release.Name == "" {
				return release.TagName
			}
			return release.Name
		}(),
		Body:        formattedBody,
		RawBody:     rawBody,
		Language:    int(language),
		PreRelease:  release.Prerelease,
		PublishedAt: release.PublishedAt,
	}

	if err = s.db.SaveRelease(&releaseToSave); err != nil {
		return fmt.Errorf("failed to save release: %w", err)
	}

	return nil
}
