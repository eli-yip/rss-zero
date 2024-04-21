package parse

import (
	"fmt"

	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/pkg/request"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"

	"go.uber.org/zap"
)

type Imager interface {
	// ParseImages will replace raw image links among text to local url
	// 	- text: text string in markdown format
	// 	- id: content id, like zhihu answer id
	//  - t: content type, see more in github.com/eli-yip/rss-zero/pkg/common.type.go
	ParseImages(text string, id, t int, logger *zap.Logger) (result string, err error)
}

// Online image parser will download images from websites,
// and save them to file service and database
type OnlineImageParser struct {
	request request.Requester
	file    file.File
	db      db.DB
	logger  *zap.Logger
}

func NewOnlineImageParser(requestService request.Requester, fileService file.File,
	dbService db.DB, logger *zap.Logger) Imager {
	return &OnlineImageParser{
		request: requestService,
		file:    fileService,
		db:      dbService,
		logger:  logger,
	}
}

// ParseImages download images and replace image links in markdown content
func (p *OnlineImageParser) ParseImages(text string, id int, t int, logger *zap.Logger) (result string, err error) {
	logger.Info("Start to parse images for zhihu content", zap.Int("content_id", id))
	for _, imageLink := range findImageLinks(text) {
		logger := logger.With(zap.String("url", imageLink))
		logger.Info("Start to save image")

		picID := URLToID(imageLink) // generate a unique int id from url by hash
		logger.Info("Generate Pic ID From url", zap.Int("Pic ID", picID))

		resp, err := p.request.NoLimitStream(imageLink)
		if err != nil {
			logger.Error("Fail to get image stream")
			return "", err
		}
		logger.Info("get image stream succussfully")

		const zhihuImageObjectKeyLayout = "zhihu/%d.jpg"
		objectKey := fmt.Sprintf(zhihuImageObjectKeyLayout, picID)
		if err = p.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
			logger.Error("Fail to save image stream to file service")
			return "", err
		}
		logger.Info("save image stream to file service successfully", zap.String("object_key", objectKey))

		if err = p.db.SaveObjectInfo(&db.Object{
			ID:              picID,
			Type:            db.ObjectTypeImage,
			ContentID:       id,
			ContentType:     t,
			ObjectKey:       objectKey,
			URL:             imageLink,
			StorageProvider: []string{p.file.AssetsDomain()},
		}); err != nil {
			logger.Error("Fail to save object info to db")
			return "", err
		}
		logger.Info("save object info to db successfully")

		objectURL := fmt.Sprintf("%s/%s", p.file.AssetsDomain(), objectKey)
		text = replaceImageLink(text, objectKey, imageLink, objectURL)
		logger.Info("replace image link successfully", zap.String("object_url", objectURL))
	}
	logger.Info("parse all images successfully", zap.Int("content id", id))
	return text, nil
}

// Offline image parser will get image info directly from database,
// and replace image links among text with local url.
type OfflineImageParser struct {
	db     db.DB
	logger *zap.Logger
}

func NewOfflineImageParser(dbService db.DB, logger *zap.Logger) Imager {
	return &OfflineImageParser{db: dbService, logger: logger}
}

func (p *OfflineImageParser) ParseImages(text string, id int, t int, logger *zap.Logger) (result string, err error) {
	logger.Info("Start to parse images for zhihu content", zap.Int("content_id", id))
	for _, imageLink := range findImageLinks(text) {
		logger := logger.With(zap.String("url", imageLink))
		picID := URLToID(imageLink)
		object, err := p.db.GetObjectInfo(picID)
		if err != nil {
			logger.Error("fail to get object info from db", zap.Error(err))
			return "", err
		}

		objectURL := fmt.Sprintf("%s/%s", object.StorageProvider[0], object.ObjectKey)
		text = replaceImageLink(text, object.ObjectKey, imageLink, objectURL)
	}
	logger.Info("parse all images successfully", zap.Int("content id", id))
	return text, nil
}
