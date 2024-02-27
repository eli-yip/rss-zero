package parse

import (
	"fmt"

	"github.com/eli-yip/rss-zero/pkg/file"
	"github.com/eli-yip/rss-zero/pkg/request"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"go.uber.org/zap"
)

type Imager interface {
	ParseImages(text string, id int, t int, logger *zap.Logger) (result string, err error)
}

type ImageParserOnline struct {
	request request.Requester
	file    file.File
	db      db.DB
	logger  *zap.Logger
}

func NewImageParserOnline(r request.Requester, f file.File,
	db db.DB, l *zap.Logger) Imager {
	return &ImageParserOnline{
		request: r,
		file:    f,
		db:      db,
		logger:  l,
	}
}

// ParseImages download images and replace image links in markdown content
func (p *ImageParserOnline) ParseImages(text string, id int, t int, logger *zap.Logger) (result string, err error) {
	for _, l := range findImageLinks(text) {
		logger := logger.With(zap.String("url", l))
		picID := URLToID(l) // generate a unique int id from url by hash

		resp, err := p.request.NoLimitStream(l)
		if err != nil {
			return "", err
		}
		logger.Info("get image stream succussfully")

		const zhihuImageObjectKeyLayout = "zhihu/%d.jpg"
		objectKey := fmt.Sprintf(zhihuImageObjectKeyLayout, picID)
		if err = p.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
			return "", err
		}
		logger.Info("save image stream to file service successfully", zap.String("object_key", objectKey))

		if err = p.db.SaveObjectInfo(&db.Object{
			ID:              picID,
			Type:            db.ObjectTypeImage,
			ContentID:       id,
			ContentType:     t,
			ObjectKey:       objectKey,
			URL:             l,
			StorageProvider: []string{p.file.AssetsDomain()},
		}); err != nil {
			return "", err
		}
		logger.Info("save object info to db successfully")

		objectURL := fmt.Sprintf("%s/%s", p.file.AssetsDomain(), objectKey)
		text = replaceImageLink(text, objectKey, l, objectURL)
		logger.Info("replace image link successfully", zap.String("object_url", objectURL))
	}
	logger.Info("parse images successfully")
	return text, nil
}

type ImageParserOffline struct {
	db     db.DB
	logger *zap.Logger
}

func NewImageParserOffline(db db.DB, l *zap.Logger) Imager {
	return &ImageParserOffline{db: db, logger: l}
}

func (p *ImageParserOffline) ParseImages(text string, id int, t int, logger *zap.Logger) (result string, err error) {
	for _, l := range findImageLinks(text) {
		logger := logger.With(zap.String("url", l))
		picID := URLToID(l)
		object, err := p.db.GetObjectInfo(picID)
		if err != nil {
			logger.Error("fail to get object info from db", zap.Error(err))
			return "", err
		}

		objectURL := fmt.Sprintf("%s/%s", object.StorageProvider[0], object.ObjectKey)
		text = replaceImageLink(text, object.ObjectKey, l, objectURL)
		logger.Info("replace image link successfully", zap.String("object_url", objectURL))
	}
	return text, nil
}
