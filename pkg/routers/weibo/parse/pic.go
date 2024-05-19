//lint:file-ignore U1000 Ignore unused function temporarily for development
package parse

import (
	"fmt"

	"github.com/eli-yip/rss-zero/pkg/routers/weibo/db"
)

func (ps *ParseService) buildObjectKey(picID string) (key string, err error) {
	return fmt.Sprintf("weibo-test/%s", picID), nil
}

func (ps *ParseService) savePicInfo(weiboID int, picID, picURL, objectKey string) (err error) {
	if err = ps.dbService.SaveObjectInfo(&db.Object{
		ID:              picID,
		Type:            db.ObjectTypeImage,
		ContentID:       weiboID,
		ObjectKey:       objectKey,
		URL:             picURL,
		StorageProvider: ps.fileService.AssetsDomain(),
	}); err != nil {
		return fmt.Errorf("failed to save pic info to db: %w", err)
	}
	return nil
}

func (ps *ParseService) savePic(picURL, objectKey string) (err error) {
	resp, err := ps.requestService.GetPicStream(picURL)
	if err != nil {
		return fmt.Errorf("failed to get pic stream: %w", err)
	}

	if err = ps.fileService.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
		return fmt.Errorf("failed to save image stream to file service: %w", err)
	}

	return nil
}
