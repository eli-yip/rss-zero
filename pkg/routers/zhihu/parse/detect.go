package parse

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/eli-yip/rss-zero/internal/ai"
)

// classifyPromptTmpl is the single, fixed classification prompt. The structure and
// the JSON output contract are global; only the injected SKIP CRITERIA varies per
// author (see detectCriteria). The two %s are: criteria, content.
const classifyPromptTmpl = `You are a content classifier. Decide whether the content below matches the following SKIP CRITERIA.

SKIP CRITERIA:
"""%s"""

CONTENT:
"""%s"""

Reply with ONLY a JSON object, no other text:
{"skip": <true|false>, "reason": "<short reason>"}`

// detectCriteria is both the enabled-author list and each author's skip criteria.
// The key is the author url_token; the value is the criteria text injected into
// classifyPromptTmpl. A missing key means "do not detect" for that author.
//
// Pure static data, maintained by the maintainer. Example shape:
//
//	"some-author-url-token": "答案主要为带货、推广或广告内容",
var detectCriteria = map[string]string{
	"shuo-shuo-98-12": "答案内容涉及答主自身的男同性恋经历、答主表达和同性恋（同志）相关的观点、答主表达和任何歌手（例如单依纯）有关的观点",
}

// DetectResult is the uniform output of content detection across all authors.
type DetectResult struct {
	Skip   bool   `json:"skip"`
	Reason string `json:"reason"`
}

// detectMaxRetry bounds transient-error retries on the AI call. Kept small because
// detection runs synchronously in the parse path.
const detectMaxRetry = 3

// ContentDetector classifies answer content via the AI service. Its only external
// dependency is ai.AI (already an interface with a mock), so it is a plain struct
// rather than an interface itself.
type ContentDetector struct {
	ai       ai.AI
	criteria map[string]string
}

// NewContentDetector builds a detector backed by the package-level detectCriteria.
func NewContentDetector(aiService ai.AI) *ContentDetector {
	return &ContentDetector{ai: aiService, criteria: detectCriteria}
}

// Detect classifies text for the given author.
//   - detected is false when the author is not registered for detection; res/err are
//     then zero/nil and the caller should leave the answer unflagged.
//   - When detected is true and err is non-nil, the caller fails open (does not skip).
func (d *ContentDetector) Detect(authorID, text string) (res DetectResult, detected bool, err error) {
	criteria, ok := d.criteria[authorID]
	if !ok {
		return DetectResult{}, false, nil
	}

	prompt := fmt.Sprintf(classifyPromptTmpl, criteria, text)

	var reply string
	for i := range detectMaxRetry {
		reply, err = d.ai.Classify(prompt)
		if err == nil {
			break
		}
		if i < detectMaxRetry-1 {
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}
	if err != nil {
		return DetectResult{}, true, fmt.Errorf("failed to classify content: %w", err)
	}

	res, err = parseDetectReply(reply)
	if err != nil {
		return DetectResult{}, true, err
	}
	return res, true, nil
}

// parseDetectReply unmarshals the model reply into a DetectResult, tolerating a
// surrounding markdown code fence that some models add.
func parseDetectReply(reply string) (DetectResult, error) {
	s := strings.TrimSpace(reply)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	var res DetectResult
	if err := json.Unmarshal([]byte(s), &res); err != nil {
		return DetectResult{}, fmt.Errorf("failed to unmarshal detect reply %q: %w", reply, err)
	}
	return res, nil
}
