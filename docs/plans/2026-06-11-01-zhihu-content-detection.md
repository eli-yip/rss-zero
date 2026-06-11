# PLAN: Zhihu Answer AI Content Detection & RSS Skipping

- Date: 2026-06-11
- SPEC: [specs/2026-06-11-01-zhihu-content-detection.md](../specs/2026-06-11-01-zhihu-content-detection.md)
- Branch: `feat/zhihu-content-detection`

## Recon summary (verified against current code)

- `zhihuDB.Answer` is registered in `internal/migrate/db.go:25` AutoMigrate, so the new
  columns are created automatically at startup — no hand-written migration.
- Both `ParseAnswer` call sites go through `parse.InitParser`
  (`pkg/routers/zhihu/cron/crawl.go:402`, `internal/controller/parse/zhihu.go:50`),
  which already receives `ai.AI`. Wiring the detector inside `InitParser` covers both.
- Other `NewParseService(WithDB(...))` call sites
  (`internal/controller/zhihu/rss.go:258`, `xiaobot/*`) only use `ParseAuthorName` /
  RSS — they never call `ParseAnswer`, so a nil detector there is safe and cannot
  clobber detect flags.
- `SaveAnswer` is `d.Save(a)` (full upsert). Flag-clobber risk is low: registered
  authors always run detection on (re)parse; unregistered authors stay
  `DetectStatusNone`. We follow the SPEC (no carry-forward); removing an author from
  the criteria map intentionally lets a later reparse reset its flag to None (shown).

## Steps

Each step lists files, the change, and how to verify. Steps are ordered by dependency.

### Step 1 — DB model: detect columns + enum

- File: `pkg/routers/zhihu/db/answer.go`
- Add to `Answer` struct:
  ```go
  DetectStatus int    `gorm:"column:detect_status;type:int;default:0"`
  DetectReason string `gorm:"column:detect_reason;type:text"`
  ```
- Add enum constants near `AnswerStatus*`:
  ```go
  const (
      DetectStatusNone = iota // 0 not detected (default / unregistered / historical)
      DetectStatusPassed      // 1 detected, passed
      DetectStatusSkipped     // 2 detected, hit, hidden from RSS
      DetectStatusFailed      // 3 detection errored; fail-open, shown
  )
  ```
- Verify: `go build ./...`; AutoMigrate adds columns on next boot (manual check
  optional via a local DB).

### Step 2 — `ai.AI`: generic `Classify`

- Files: `internal/ai/ai.go`, `internal/ai/gpt.go`
- Add to the `AI` interface: `Classify(prompt string) (reply string, err error)`.
- `AIService.Classify` (gpt.go): `return a.askGPT(prompt)`.
- `AIServiceWithoutAPI.Classify`: `return \`{"skip": false}\`, nil` (mock never skips).
- Verify: `go build ./...`; `go test ./internal/ai/...` if a cheap test fits.

### Step 3 — ContentDetector (new file)

- File: `pkg/routers/zhihu/parse/detect.go` (new)
- Contents:
  - `DetectResult{ Skip bool; Reason string }`.
  - `classifyPromptTmpl` const (fixed template with two `%s`: criteria, content;
    instructs JSON-only output per SPEC §5).
  - `detectCriteria map[string]string` — placeholder entries; maintainer fills in
    (SPEC §9). Keep one commented placeholder key so the shape is obvious.
  - `ContentDetector{ ai ai.AI; criteria map[string]string }`.
  - `NewContentDetector(ai ai.AI) *ContentDetector` → uses package `detectCriteria`.
  - `Detect(authorID, text string) (res DetectResult, detected bool, err error)`:
    - `criteria, ok := d.criteria[authorID]`; if `!ok` return `_, false, nil`.
    - `reply, err := d.ai.Classify(fmt.Sprintf(classifyPromptTmpl, criteria, text))`.
    - On err: return `DetectResult{}, true, err` (caller fail-opens).
    - Unmarshal `reply` into `DetectResult` (tolerant: trim fencing/whitespace; a
      parse error returns `true, err` → caller fail-opens to `DetectStatusFailed`).
  - Retry: wrap the `ai.Classify` call in a small bounded retry on transient errors,
    mirroring the encryption-service 502 retry style (commit `fa552d3`). Keep it
    local and simple (e.g. 3 attempts).
- Verify: covered by Step 8 tests; `go build ./...`.

### Step 4 — Wire detector into ParseService

- File: `pkg/routers/zhihu/parse/parse.go`
- Add field `detector *ContentDetector` to `ParseService`.
- Add option `WithContentDetector(d *ContentDetector) Option`.
- In `InitParser`, construct `NewContentDetector(aiService)` and pass
  `WithContentDetector(detector)` alongside the existing options. Both ParseAnswer
  paths inherit it; direct `NewParseService(WithDB(...))` callers stay nil.
- Verify: `go build ./...`.

### Step 5 — Detection in ParseAnswer

- File: `pkg/routers/zhihu/parse/answer.go`
- After `formattedText` (L106) and before `SaveAnswer` (L121), insert the block from
  SPEC §6 (status switch on `detected` / err / `res.Skip`).
- Pass `DetectStatus: detectStatus, DetectReason: detectReason` into the `db.Answer`
  literal in `SaveAnswer`.
- Verify: `go build ./...`; Step 8 integration test.

### Step 6 — RSS-dedicated visible queries

- File: `pkg/routers/zhihu/db/answer.go` (+ interface in same file, + mock in
  `pkg/routers/zhihu/db/db_mock.go`)
- Add to `DBAnswer` interface and implement:
  ```go
  GetLatestNVisibleAnswer(n int, userID string) ([]Answer, error)
  GetVisibleAnswerAfter(userID string, t time.Time) ([]Answer, error)
  ```
  Same bodies as `GetLatestNAnswer` / `GetAnswerAfter` plus
  `.Where("detect_status <> ?", DetectStatusSkipped)`.
- Do NOT modify the existing `GetLatestNAnswer` / `GetAnswerAfter` (reused by crawl
  cursor `cron/crawl.go:282` and statistics `controller/archive/statistics.go:31`).
- Add the two methods to `MockDB`.
- Verify: `go build ./...`.

### Step 7 — RSS generation uses visible queries

- File: `internal/rss/zhihu.go`
- In `generateZhihuAnswer` (L70-116): swap `GetLatestNAnswer` → `GetLatestNVisibleAnswer`
  and `GetAnswerAfter` → `GetVisibleAnswerAfter` (both the primary and the
  empty-fallback calls). Article/Pin paths unchanged.
- Verify: `go build ./...`; `go test ./internal/rss/...` (targeted).

### Step 8 — Tests (unit only, no DB)

Per maintainer decision: **no database-related integration tests.** Cover only the
pure logic that needs no DB:

- `pkg/routers/zhihu/parse/detect_test.go`: table tests for `Detect` using a fake
  `ai.AI` returning canned replies — hit, non-hit, malformed JSON (fail-open),
  ai error (fail-open), unregistered author (`detected=false`).
- Skip: parse-integration (SaveAnswer) and visible-query WHERE-clause tests — both
  require a real DB.
- Verify: `go test ./pkg/routers/zhihu/parse/...` (targeted, per AGENTS.md).

### Step 9 — Docs bookkeeping

- Update `docs/PROGRESS.md` status as steps land.
- Maintain `docs/lessons/2026-06-11-01-zhihu-content-detection.md`: append notes while
  executing; reorganize into a summary once the plan completes (per AGENTS.md).

## Out of scope (per SPEC §9)

- Historical backfill of existing answers.
- Authoring the real `detectCriteria` values (maintainer-provided).
- article / pin detection.

## Verification at the end

- `go build ./...` clean.
- `go test ./pkg/routers/zhihu/parse/...` green (detector unit tests).
- No DB integration tests (maintainer decision).
