package tombkeeper

import (
	"fmt"

	"github.com/samber/lo"
)

// ContentSnapshot 是渲染一组根帖所需的自包含只读内容。
type ContentSnapshot struct {
	Posts  map[int64]Post
	Images map[string]ImageAsset
}

// ContentLoader 批量装配根帖、一层直接引用及其图片资产。
type ContentLoader struct{ reader ContentReader }

func NewContentLoader(reader ContentReader) *ContentLoader { return &ContentLoader{reader: reader} }

func (l *ContentLoader) Load(roots []Post) (ContentSnapshot, error) {
	content := ContentSnapshot{
		Posts:  make(map[int64]Post, len(roots)),
		Images: make(map[string]ImageAsset),
	}
	for _, post := range roots {
		content.Posts[post.ID] = post
	}
	refSet := make(map[int64]struct{})
	for _, post := range roots {
		for _, id := range referencePostIDs(post.RetweetPostID, post.Links) {
			if _, root := content.Posts[id]; !root {
				refSet[id] = struct{}{}
			}
		}
	}
	refs := lo.Keys(refSet)
	referenced, err := l.reader.GetPosts(refs)
	if err != nil {
		return ContentSnapshot{}, fmt.Errorf("load directly referenced posts: %w", err)
	}
	for _, post := range referenced {
		content.Posts[post.ID] = post
	}

	imageSet := make(map[string]struct{})
	for _, post := range content.Posts {
		for _, rawImage := range post.Pics {
			if id := picIDOf(rawImage); id != "" {
				imageSet[id] = struct{}{}
			}
		}
		for _, ids := range post.H5ImageIDsByURL {
			for _, id := range ids {
				imageSet[id] = struct{}{}
			}
		}
	}
	imageIDs := lo.Keys(imageSet)
	assets, err := l.reader.GetImageAssets(imageIDs)
	if err != nil {
		return ContentSnapshot{}, fmt.Errorf("load image assets: %w", err)
	}
	for _, asset := range assets {
		content.Images[asset.ID] = asset
	}
	return content, nil
}
