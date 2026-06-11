# SPEC: Zhihu Answer AI Content Detection & RSS Skipping

- Date: 2026-06-11
- Status: Accepted
- Scope: `zhihu_answer` only

## 1. Goal

After a Zhihu answer is parsed into markdown, if its `AuthorID` is registered for
detection, synchronously run an AI classification. Answers judged as "hit" are
**still stored, but flagged**, and are filtered out when the RSS feed is generated.

## 2. Scope

- Applies to `zhihu_answer` only. `zhihu_article` and `zhihu_pin` are untouched.
- Detection runs only for authors registered in the detector registry. Authors not
  in the registry behave exactly as before.

## 3. Data model changes

Add two columns to the `Answer` struct in `pkg/routers/zhihu/db/answer.go`. GORM
`AutoMigrate` adds the columns automatically; the defaults are backward compatible
with existing rows.

```go
DetectStatus int    `gorm:"column:detect_status;type:int;default:0"`
DetectReason string `gorm:"column:detect_reason;type:text"`
```

Status enum (new constants):

```go
const (
    DetectStatusNone    = iota // 0 not detected (historical rows / unregistered author; default)
    DetectStatusPassed         // 1 detected, passed, shown normally
    DetectStatusSkipped        // 2 detected, hit, hidden from RSS
    DetectStatusFailed         // 3 detection errored; fail-open, still shown; eligible for re-check
)
```

RSS filter rule: **only `DetectStatus == DetectStatusSkipped` (2) is hidden.** All
other states (0/1/3) are shown — consistent with fail-open.

## 4. Detection: fixed template + per-author criteria

The classification **prompt structure and output contract are fixed**. The only thing
that varies per author is *what counts as a hit* — the skip **criteria**. So we do not
carry a full prompt per author; we carry only the criteria text, injected into a
single shared prompt template (§5).

The whole thing is **one concrete struct**, `ContentDetector`. The only external
dependency — and the only real test seam — is `ai.AI`, which is already an interface
with a mock (`AIServiceWithoutAPI`). There is no second implementation of the
criteria lookup or the classification logic, so neither is made an interface (no
`CriteriaRegistry`/`Classifier` interfaces — that would be ceremony without
substitution value).

```go
// Uniform output across all authors
type DetectResult struct {
    Skip   bool
    Reason string
}

// detectCriteria is the enabled-author list plus each author's skip criteria.
// Pure static data; a missing key means "do not detect".
var detectCriteria = map[string]string{
    "author-a-url-token": "<skip criteria for author A>", // placeholder, see §9
    "author-b-url-token": "<skip criteria for author B>", // placeholder, see §9
}

type ContentDetector struct {
    ai       ai.AI             // only external dependency; already an interface (+ mock)
    criteria map[string]string // injected; defaults to detectCriteria
}

// Detect returns detected=false when the author is not registered (skip detection).
func (d *ContentDetector) Detect(authorID, text string) (res DetectResult, detected bool, err error) {
    criteria, ok := d.criteria[authorID]
    if !ok {
        return DetectResult{}, false, nil
    }
    reply, err := d.ai.Classify(fmt.Sprintf(classifyPromptTmpl, criteria, text))
    if err != nil {
        return DetectResult{}, true, err
    }
    // unmarshal reply (JSON contract, §5) -> res
    return res, true, nil
}
```

`ContentDetector` is injected into `ParseService` via an Option (mirroring `WithAI`):
`WithContentDetector(d)`, stored in a `detector *ContentDetector` field. A `nil`
field means detection is disabled (consistent with the existing nil-checks).

The real test seam is `ai.AI`: a parse test injects a fake `ai.AI` that returns canned
JSON to deterministically drive hit / non-hit, with no interface needed on the
detector itself.

## 5. Fixed prompt template & JSON output contract

The single shared template lives next to the detector (domain layer), with the
criteria and content injected:

```go
const classifyPromptTmpl = `You are a content classifier. Decide whether the content
below matches the following SKIP CRITERIA.

SKIP CRITERIA:
"""%s"""

CONTENT:
"""%s"""

Reply with ONLY a JSON object, no other text:
{"skip": <true|false>, "reason": "<short reason>"}`
```

Add one generic low-level method to the `ai.AI` interface (reusing the existing
private `askGPT`); the detector builds the full prompt and calls it:

```go
Classify(prompt string) (reply string, err error)
```

The detector `fmt.Sprintf`s the template with `(criteria, text)`, calls `ai.Classify`,
and unmarshals the reply into `DetectResult`:

```json
{ "skip": true, "reason": "short reason string" }
```

- `skip` (bool): true means the content is a "hit" and should be hidden from RSS.
- `reason` (string): short human-readable explanation, stored in `DetectReason`.

Template, output contract, and JSON parsing are global and shared; only the injected
criteria string differs per author.

`AIServiceWithoutAPI` (the no-API-key mock) returns `{"skip": false}` so test/no-key
environments never falsely skip content.

## 6. Parse flow change

In `pkg/routers/zhihu/parse/answer.go` `ParseAnswer`, after `formattedText` is
produced (currently L106) and before `SaveAnswer` (currently L121):

```go
detectStatus := DetectStatusNone
detectReason := ""
if p.detector != nil {
    res, detected, derr := p.detector.Detect(authorID, formattedText)
    switch {
    case !detected:
        // author not registered; leave DetectStatusNone
    case derr != nil:
        logger.Error("content detect failed, fail-open", zap.Error(derr))
        detectStatus = DetectStatusFailed // fail-open: do not skip
    case res.Skip:
        detectStatus = DetectStatusSkipped
        detectReason = res.Reason
    default:
        detectStatus = DetectStatusPassed
    }
}
```

Pass `DetectStatus: detectStatus, DetectReason: detectReason` into `SaveAnswer`.

- **Synchronous**: detection completes inside the parse path, so the row is stored
  with an accurate flag.
- On failure, fail-open: record `DetectStatusFailed`, eligible for future re-check.

## 7. RSS generation change

Do **not** modify `GetLatestNAnswer` / `GetAnswerAfter` — they are reused by the crawl
cursor (`cron/crawl.go`) and by statistics (`controller/archive/statistics.go`).
Add two RSS-dedicated query methods that add a single `WHERE detect_status <> 2`:

```go
GetLatestNVisibleAnswer(n int, userID string) ([]Answer, error)
GetVisibleAnswerAfter(userID string, t time.Time) ([]Answer, error)
```

`internal/rss/zhihu.go` `generateZhihuAnswer` (L70-116) switches to these two methods.
The article/pin paths are unchanged.

Rationale for dedicated methods over loop-side filtering: skipped answers do not eat
into the `DefaultFetchCount` quota, so the RSS item count stays accurate.

## 8. Boundaries

1. **Re-parse**: a newer `update_at` re-parses the answer and re-detects it,
   overwriting the previous flag. Expected.
2. **Retry**: the detector retries a few times on AI 5xx / network errors (mirroring
   commit `fa552d3`, encryption-service 502 retry) before failing open.
3. **Unregistered authors**: never detected; `DetectStatus` stays 0; shown normally.

## 9. Decisions & open items

- **Historical backfill — out of scope.** Existing answers keep `detect_status = 0`
  and are shown normally. Detection applies only to answers parsed after this feature
  ships. No backfill command/migration.
- **Author criteria — provided by the maintainer.** The `detectCriteria` entries
  (author `url_token` keys + skip-criteria values) are authored separately. The
  skeleton ships with placeholder criteria; the map is filled in by the maintainer.
