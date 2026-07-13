package parse

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/samber/lo"
)

type PinParser interface {
	ParsePinList(content []byte, index int, logger *zap.Logger) (paging apiModels.Paging, pinsExcerpt []apiModels.Pin, pins []json.RawMessage, err error)
	ParsePin(content []byte, logger *zap.Logger) error
}

func (p *ParseService) ParsePinList(content []byte, index int, logger *zap.Logger) (paging apiModels.Paging, pinsExcerpt []apiModels.Pin, pins []json.RawMessage, err error) {
	logger.Info("Start to parse pin list", zap.Int("pin_list_page_index", index))

	pinList := apiModels.PinList{}
	if err = json.Unmarshal(content, &pinList); err != nil {
		logListPayloadDiagnostics(logger, "pin", index, content, err)
		return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to unmarshal content data in to pin list: %w", err)
	}
	logger.Info("Unmarshal pin list successfully",
		zap.Int("data_count", len(pinList.Data)),
		zap.Int("paging_total", pinList.Paging.Totals),
		zap.Bool("is_end", pinList.Paging.IsEnd))

	for _, rawMessage := range pinList.Data {
		pin := apiModels.Pin{}
		if err = json.Unmarshal(rawMessage, &pin); err != nil {
			return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to unmarshal data in to pin: %w", err)
		}
		pinsExcerpt = append(pinsExcerpt, pin)
	}

	return pinList.Paging, pinsExcerpt, pinList.Data, nil
}

// ParsePin parses the zhihu.com/api/v4 resp
func (p *ParseService) ParsePin(content []byte, logger *zap.Logger) (err error) {
	pin := apiModels.Pin{}
	if err = json.Unmarshal(content, &pin); err != nil {
		return fmt.Errorf("failed to unmarshal content data in to pin: %w", err)
	}
	pinID, err := strconv.Atoi(pin.ID)
	if err != nil {
		return fmt.Errorf("failed to convert pin id to int: %w", err)
	}
	logger.Info("Unmarshal pin successfully")

	pinInDB, err := loadOrAbsent(p.db.GetPin, pinID)
	if err != nil {
		return fmt.Errorf("failed to get pin from db: %w", err)
	}
	if pinInDB != nil && storedIsCurrent(pinInDB.UpdateAt, time.Unix(pin.UpdateAt, 0)) {
		logger.Info("Pin already up-to-date, skip re-parsing")
		return nil
	}

	result, err := p.buildPinResult(&pin, content, pinID, logger)
	if err != nil {
		return fmt.Errorf("failed to parse pin: %w", err)
	}
	if result == nil {
		logger.Info("Pin has no content, skip")
		return nil
	}

	// 原子提交：整棵 pin 树的对象 + 作者 + 各 pin 根行同一事务，一起提交或一起回滚（plan 决策 4）；
	// 事务内根行最后写只是可读性约定，无 FK 强制、不改变回滚语义。
	if err = p.db.SavePinTx(result.Pins, result.Authors, result.Objects); err != nil {
		return fmt.Errorf("failed to save pin to db: %w", err)
	}
	logger.Info("Save pin to db successfully")

	return nil
}

// PinParseResult 汇集一条 pin 树（顶层 + origin，代码递归任意深度、zhihu 实际至多一层）解析后
// 待原子提交的全部事实行（决策 4）。buildPinResult 递归组装它，交给 db.SavePinTx 在单事务内落库；
// 原子性来自事务本身，根行最后写只是可读性约定。
type PinParseResult struct {
	Pins    []db.Pin    // 根行；origin 引用的 pin 在前、顶层在后（可读性约定，非事务约束）
	Authors []db.Author // 作者（顶层 + origin）
	Objects []db.Object // 图片对象，OSS 已上传成功
}

// buildPinResult 递归抽取一条 pin（含 origin_pin，代码递归任意深度、zhihu 实际至多一层）的
// 全部待提交事实、推导标题，装进 PinParseResult；不落库。返回 nil 表示这条 pin 空正文应 skip
// （不产出根行，plan 决策 6）。
// origin 作为独立根行随顶层同事务提交：其对象 / 作者 / 根行都并入返回结果，origin 在前、顶层在后。
func (p *ParseService) buildPinResult(pin *apiModels.Pin, content []byte, pinID int, logger *zap.Logger) (*PinParseResult, error) {
	title, text, ownObjects, err := p.parsePinContent(pin.Content, pinID, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pin content: %w", err)
	}
	logger.Info("Parse pin content successfully")

	result := &PinParseResult{Objects: ownObjects}
	// treeObjects 汇总整棵子树（本 pin + origin）的对象，供本 pin 的 transient 渲染换链。
	treeObjects := objectsByID(ownObjects)

	if pin.OriginPin != nil {
		contentBytes, err := json.Marshal(pin.OriginPin)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal origin pin: %w", err)
		}
		oPinID, err := strconv.Atoi(pin.OriginPin.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to convert origin pin id to int: %w", err)
		}
		originResult, err := p.buildPinResult(pin.OriginPin, contentBytes, oPinID, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to parse origin pin: %w", err)
		}
		// origin 空正文（originResult == nil）时其根行 skip、不入库，但顶层仍据内嵌 raw 渲染引用块
		// 并保存——与旧递归行为一致（origin 根行缺失不阻断顶层）。
		if originResult != nil {
			result.Objects = append(result.Objects, originResult.Objects...)
			result.Authors = append(result.Authors, originResult.Authors...)
			result.Pins = append(result.Pins, originResult.Pins...)
			for _, o := range originResult.Objects {
				treeObjects[o.ID] = o
			}
		}
	}

	// 空正文 skip、不产出根行：origin 存在时其引用块（含固定文案）恒非空，故旧「合并后为空」判定
	// 等价于「无 origin 且正文块为空」。
	if pin.OriginPin == nil && text == "" {
		return nil, nil
	}

	// transient 正文：喂 AI 标题结论（origin 已内嵌在 content，自包含），不持久化。
	snapshot := render.ContentSnapshot{
		Pins:    map[int]db.Pin{pinID: {ID: pinID, AuthorID: pin.Author.ID, Raw: content}},
		Objects: treeObjects,
	}
	body, err := render.RenderMarkdown(pinID, snapshot, config.C.Settings.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to render pin markdown: %w", err)
	}

	if title == "" {
		if title, err = p.ai.Conclude(body); err != nil {
			return nil, fmt.Errorf("failed to conclude pin content: %w", err)
		}
		logger.Info("Conclude pin content successfully", zap.String("title", title))
	}

	result.Authors = append(result.Authors, db.Author{ID: pin.Author.ID, Name: pin.Author.Name})
	result.Pins = append(result.Pins, db.Pin{
		ID:       pinID,
		AuthorID: pin.Author.ID,
		CreateAt: time.Unix(pin.CreateAt, 0),
		UpdateAt: time.Unix(pin.UpdateAt, 0),
		Title:    title,
		Raw:      content,
	})
	return result, nil
}

// parsePinContent 逐块处理一条 pin 的内容：抽取标题、拼出用于「空正文 skip」判定的正文文本，
// 并逐个下载图片、转存 OSS（事务外网络副作用）、把对象元数据收进返回切片——不落库、不换链。
// 返回的 text 只用于标题切分与空正文判定，绝不落库；正文由读取期纯渲染重放（见 render 包）。
func (p *ParseService) parsePinContent(content []json.RawMessage, id int, logger *zap.Logger) (title, text string, objects []db.Object, err error) {
	textPart := make([]string, 0)

	for _, c := range content {
		var contentType apiModels.PinContentType
		if err = json.Unmarshal(c, &contentType); err != nil {
			return emptyString, emptyString, nil, fmt.Errorf("failed to unmarshal content type: %w", err)
		}

		switch contentType.Type {
		case "text":
			logger.Info("Found text content")

			var textContent apiModels.PinContentText
			if err = json.Unmarshal(c, &textContent); err != nil {
				return emptyString, emptyString, nil, fmt.Errorf("failed to unmarshal text content: %w", err)
			}
			textBytes, err := p.htmlToMarkdown.Convert([]byte(textContent.Content))
			if err != nil {
				return emptyString, emptyString, nil, fmt.Errorf("failed to convert html to markdown: %w", err)
			}
			text := string(textBytes) // 块内局部，勿复用上一个块的 text（#6）
			title, text = render.TryToFindTitle(text)

			textPart = append(textPart, text)

			logger.Info("Convert text part to markdown successfully")
		case "image":
			logger.Info("Found image content")
			var imageContent apiModels.PinImage
			if err = json.Unmarshal(c, &imageContent); err != nil {
				return emptyString, emptyString, nil, fmt.Errorf("failed to unmarshal image content: %w", err)
			}
			picID := render.URLToID(imageContent.OriginalURL)

			resp, err := p.GetImageStream(imageContent.OriginalURL, logger)
			if err != nil {
				return emptyString, emptyString, nil, fmt.Errorf("failed to get image %s stream: %w", imageContent.OriginalURL, err)
			}
			logger.Info("Get image stream succussfully")

			const zhihuImageObjectKeyLayout = "zhihu/%d.jpg"
			objectKey := fmt.Sprintf(zhihuImageObjectKeyLayout, picID)
			if err = p.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
				return emptyString, emptyString, nil, fmt.Errorf("failed to save image stream %s to file service: %w", imageContent.OriginalURL, err)
			}
			logger.Info("Save image stream to file service successfully", zap.String("object_key", objectKey))

			// 对象元数据不即时写库，收进待提交切片，随根行同事务落库（plan 决策 4）。
			objects = append(objects, db.Object{
				ID:              picID,
				Type:            db.ObjectTypeImage,
				ContentType:     common.ZhihuPin,
				ContentID:       id,
				ObjectKey:       objectKey,
				URL:             imageContent.OriginalURL,
				StorageProvider: []string{p.file.AssetsDomain()},
			})

			// 仍拼出图片块占位文本并入 textPart：只用于「空正文 skip」判定与标题切分，不落库/不换链。
			objectURL := fmt.Sprintf("%s/%s", p.file.AssetsDomain(), objectKey)
			text = fmt.Sprintf("![%s](%s)", objectKey, objectURL)

			textPart = append(textPart, text)

			logger.Info("Convert image to markdown successfully")
		case "link":
			logger.Info("Found link content")

			var linkContent apiModels.PinLink
			if err := json.Unmarshal(c, &linkContent); err != nil {
				return emptyString, emptyString, nil, fmt.Errorf("failed to unmarshal link content: %w", err)
			}
			text = fmt.Sprintf("[%s](%s)", linkContent.Title, linkContent.URL)

			textPart = append(textPart, text)
		case "video":
			logger.Info("Found video content")

			var videoContent apiModels.PinVideo
			if err := json.Unmarshal(c, &videoContent); err != nil {
				return emptyString, emptyString, nil, fmt.Errorf("failed to unmarshal video content: %w", err)
			}
			logger.Info("Unmarshal video content successfully")

			maxVideo := lo.MaxBy(videoContent.Playlist, func(a, b apiModels.PlaylistItem) bool { return a.Size > b.Size })
			logger.Info("Found max video", zap.Any("max_video", maxVideo))

			videoURL := maxVideo.Url
			videoID := videoContent.VideoID
			logger.Info("Found video", zap.String("video_url", videoURL), zap.String("video_id", videoID))

			text = fmt.Sprintf("![视频 %s](%s)", videoID, videoURL)
			textPart = append(textPart, text)
		case "link_card":
			logger.Info("Found link card content")

			var linkCardContent apiModels.PinLinkCard
			if err := json.Unmarshal(c, &linkCardContent); err != nil {
				return emptyString, emptyString, nil, fmt.Errorf("failed to unmarshal link card content: %w", err)
			}

			text = fmt.Sprintf("[%s|%s](%s)", linkCardContent.DataContentType, linkCardContent.URL, linkCardContent.URL)
			textPart = append(textPart, text)
		case "poll":
			logger.Info("Found poll content")
		default:
			return emptyString, emptyString, nil, fmt.Errorf("unknown content type: %s", contentType.Type)
		}
	}

	text = md.Join(textPart...)
	return title, text, objects, nil
}
