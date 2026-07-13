package parse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	commonRender "github.com/eli-yip/rss-zero/pkg/render"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
)

// TopicParseResult 汇集一条 topic 解析后待原子提交的全部事实行（决策 4）。
// ParseTopic 组装它，交给 db.SaveTopicTx 在单事务内落库、根行最后写。
// 与 models.TopicParseResult（抓取期承载 API 载荷的工作结构）不同，这里全是 db 行。
type TopicParseResult struct {
	Topic   db.Topic    // 根行，事务内最后写
	Author  *db.Author  // 作者；未知类型为 nil
	Article *db.Article // 外部文章；仅 talk 且带 article 时非 nil
	Objects []db.Object // 文件 / 图片 / 语音，OSS 已上传成功
}

// topicIDSkip lists topics that are skipped during parsing because their
// article content crashes / times out the markdown converter.
var topicIDSkip = map[int]struct{}{
	2855142121821411:  {},
	4848142822512458:  {}, // Cause article markdown converter error
	1525884245581542:  {}, // Cause article markdown converter error
	1524441421222552:  {}, // Same
	8852488254285212:  {}, // Same
	14588211152588222: {}, // Same
	22811844581522481: {}, // Same
	14588584821842242: {}, // Same
	14588442484115242: {}, // Same
	14588445248825242: {}, // Same
}

// SplitTopics split the api response bytes from zsxq api to raw topics
func (s *ParseService) SplitTopics(respBytes []byte, logger *zap.Logger) (rawTopics []json.RawMessage, err error) {
	logger.Info("Start to split topics")
	resp := models.APIResponse{}
	if err = json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal api response: %w", err)
	}
	logger.Info("Successfully unmarshal api response")
	return resp.RespData.RawTopics, nil
}

// ParseTopic 抽取一条 topic 的事实、推导标题，再在单事务内原子落库（根行最后写）。
// 抓取期不再持久化正文 Markdown：标题由 transient 纯渲染喂入（不落库），正文读取期从
// raw + 侧表重放。skip / error 语义见 plan 决策 6 行为矩阵，逐条保持不变。
func (s *ParseService) ParseTopic(topic *models.TopicParseResult, logger *zap.Logger) (err error) {
	if _, skip := topicIDSkip[topic.TopicID]; skip {
		logger.Info("Skip crawling topic, as it will cause markdown parser timeout", zap.Int("topic_id", topic.TopicID))
		return nil
	}

	logger.Info("Start to process topic", zap.String("type", topic.Type))

	var result TopicParseResult
	switch topic.Type {
	case "talk":
		author, objects, article, err := s.parseTalk(logger, &topic.Topic)
		if err != nil {
			switch {
			case errors.Is(err, ErrNoText):
				logger.Info("This topic has no text, skip")
				return nil
			case errors.Is(err, commonRender.ErrTimeout):
				logger.Warn("This topic's article markdown converter timeout, skip", zap.Int("topic_id", topic.TopicID))
				return nil
			default:
				return fmt.Errorf("failed to parse talk: %w", err)
			}
		}
		result.Author, result.Objects, result.Article = author, objects, article
	case "q&a":
		author, objects, err := s.parseQA(logger, &topic.Topic)
		if err != nil {
			return fmt.Errorf("failed to parse q&a: %w", err)
		}
		result.Author, result.Objects = author, objects
	default:
		logger.Info("This topic is not a talk or q&a")
	}
	logger.Info("Parse topic facts successfully")

	createTimeInTime, err := zsxqTime.DecodeZsxqAPITime(topic.CreateTime)
	if err != nil {
		return fmt.Errorf("failed to decode create time: %w", err)
	}
	logger.Info("Get topic create time successfully", zap.Time("create_time", createTimeInTime))

	authorID := 0
	if result.Author != nil {
		authorID = result.Author.ID
	}

	// 派生标题：用刚抽取的事实在内存装配快照，跑读取期同一个纯 RenderMarkdown 得临时正文，
	// 喂 AI 归纳标题——与读取期正文逐字节一致，但临时正文不持久化。未知类型无正文可渲染，
	// RenderMarkdown 返回 ErrUnknownType，按旧实现跳过标题结论。
	title := topic.Title
	body, renderErr := transientBody(topic.TopicID, authorID, topic.Type, topic.Raw, &result)
	if renderErr != nil {
		if errors.Is(renderErr, render.ErrUnknownType) {
			logger.Info("This topic is not a talk or q&a, skip title conclusion", zap.Error(renderErr))
		} else {
			return fmt.Errorf("failed to render topic to markdown text: %w", renderErr)
		}
	} else {
		logger.Info("Render topic to markdown text successfully")
		if title == nil ||
			// Zsxq API will return a excerpt with suffix "..." as title if there is no title
			strings.HasSuffix(*title, "...") {
			concluded, err := s.ai.Conclude(body)
			if err != nil {
				return fmt.Errorf("failed to conclude title: %w", err)
			}
			title = &concluded
			logger.Info("Conclude title successfully")
		}
	}

	result.Topic = db.Topic{
		ID:       topic.TopicID,
		Time:     createTimeInTime,
		GroupID:  topic.Group.GroupID,
		Type:     topic.Type,
		Digested: topic.Digested,
		AuthorID: authorID,
		Title:    title,
		Raw:      topic.Raw,
	}

	if err = s.db.SaveTopicTx(&result.Topic, result.Author, result.Article, result.Objects); err != nil {
		return fmt.Errorf("failed to save topic info to database: %w", err)
	}
	logger.Info("Save topic info to database successfully")

	return nil
}

// transientBody 用刚解析的事实在内存装配 ContentSnapshot，跑读取期同一个纯 RenderMarkdown
// 得到临时正文（不持久化），供 AI 标题结论。作者名取 db.Author.Name，与读取期一致
// （读取期 RenderMarkdown 用 Authors[AuthorID].Name，不解析别名）。
func transientBody(topicID, authorID int, topicType string, raw []byte, r *TopicParseResult) (string, error) {
	snapshot := render.ContentSnapshot{
		Topics:   map[int]db.Topic{topicID: {ID: topicID, Type: topicType, AuthorID: authorID, Raw: raw}},
		Objects:  make(map[int]db.Object, len(r.Objects)),
		Articles: map[string]db.Article{},
		Authors:  map[int]db.Author{},
	}
	for _, o := range r.Objects {
		snapshot.Objects[o.ID] = o
	}
	if r.Article != nil {
		snapshot.Articles[r.Article.ID] = *r.Article
	}
	if r.Author != nil {
		snapshot.Authors[r.Author.ID] = *r.Author
	}
	return render.RenderMarkdown(topicID, snapshot)
}

const ZsxqFileBaseURL = "https://api.zsxq.com/v2/files/%d/download_url"

type FileDownload struct {
	RespData struct {
		DownloadURL string `json:"download_url"`
	} `json:"resp_data"`
}

func (s *ParseService) downloadLink(fileID int, logger *zap.Logger) (link string, err error) {
	url := fmt.Sprintf(ZsxqFileBaseURL, fileID)

	resp, err := s.request.Limit(context.Background(), url, logger)
	if err != nil {
		return "", fmt.Errorf("failed to request zsxq api: %w", err)
	}

	download := FileDownload{}
	if err = json.Unmarshal(resp, &download); err != nil {
		return "", fmt.Errorf("failed to unmarshal download link: %w", err)
	}

	return download.RespData.DownloadURL, nil
}
