package parse

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
	"go.uber.org/zap"
)

type Imager interface {
	// ParseImages will replace raw image links among text to local url
	// 	- text: text string in markdown format
	// 	- id: content id, like zhihu answer id
	//  - t: content type, see more in github.com/eli-yip/rss-zero/pkg/common.type.go
	ParseImages(text string, id, t int, logger *zap.Logger) (result string, err error)
	GetImageStream(url string, logger *zap.Logger) (resp *http.Response, err error)
}

// Online image parser will download images from websites,
// and save them to file service and database
type OnlineImageParser struct {
	request request.Requester
	file    file.File
	db      db.DB
}

func NewOnlineImageParser(requestService request.Requester, fileService file.File, dbService db.DB) Imager {
	return &OnlineImageParser{
		request: requestService,
		file:    fileService,
		db:      dbService,
	}
}

// ParseImages download images and replace image links in markdown content
func (p *OnlineImageParser) ParseImages(text string, id int, t int, logger *zap.Logger) (result string, err error) {
	for _, imageLink := range findImageLinks(text) {
		picID := URLToID(imageLink) // generate a unique int id from url by hash

		resp, err := p.request.NoLimitStream(imageLink, logger)
		if err != nil {
			return "", fmt.Errorf("failed to get image stream for url %s: %w", imageLink, err)
		}

		const zhihuImageObjectKeyLayout = "zhihu/%d.jpg"
		objectKey := fmt.Sprintf(zhihuImageObjectKeyLayout, picID)
		if err = p.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
			return "", fmt.Errorf("failed to save image stream to file service: %w", err)
		}

		if err = p.db.SaveObjectInfo(&db.Object{
			ID:              picID,
			Type:            db.ObjectTypeImage,
			ContentID:       id,
			ContentType:     t,
			ObjectKey:       objectKey,
			URL:             imageLink,
			StorageProvider: []string{p.file.AssetsDomain()},
		}); err != nil {
			return "", fmt.Errorf("failed to save object info to db: %w", err)
		}

		objectURL := fmt.Sprintf("%s/%s", p.file.AssetsDomain(), objectKey)
		text = replaceImageLink(text, objectKey, imageLink, objectURL)
	}
	return text, nil
}

func (p *OnlineImageParser) GetImageStream(url string, logger *zap.Logger) (resp *http.Response, err error) {
	return p.request.NoLimitStream(url, logger)
}

// Offline image parser will get image info directly from database,
// and replace image links among text with local url.
type OfflineImageParser struct{ db db.DB }

func NewOfflineImageParser(dbService db.DB) Imager {
	return &OfflineImageParser{db: dbService}
}

func (p *OfflineImageParser) ParseImages(text string, id int, t int, logger *zap.Logger) (result string, err error) {
	for _, imageLink := range findImageLinks(text) {
		picID := URLToID(imageLink)
		object, err := p.db.GetObjectInfo(picID)
		if err != nil {
			return "", fmt.Errorf("failed to get object info from db: %w, url: %s", err, imageLink)
		}

		objectURL := fmt.Sprintf("%s/%s", object.StorageProvider[0], object.ObjectKey)
		text = replaceImageLink(text, object.ObjectKey, imageLink, objectURL)
	}
	return text, nil
}

func (p *OfflineImageParser) GetImageStream(url string, logger *zap.Logger) (resp *http.Response, err error) {
	return nil, errors.New("not implemented")
}
