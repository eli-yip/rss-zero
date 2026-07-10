# LESSON: Zhihu Answer AI Content Detection & RSS Skipping

- Date: 2026-06-11
- PLAN: [plans/2026-06-11-zhihu-content-detection.md](../plans/2026-06-11-zhihu-content-detection.md)
- Branch: `feat/zhihu-content-detection`

Summary written after implementation completed.

## What went smoothly

- AutoMigrate already registers `zhihuDB.Answer` (`internal/migrate/db.go`), so the
  two new columns (`detect_status`, `detect_reason`) need no hand-written migration.
- Both `ParseAnswer` call sites route through `parse.InitParser`, so wiring the
  detector once inside `InitParser` covered the crawl path and the manual-parse path
  with no per-call-site changes.

## Lessons / gotchas

- **Interface compliance is enforced at every implementation.** Adding `Classify` to
  the `ai.AI` interface required implementing it on **both** `AIService` (real) and
  `AIServiceWithoutAPI` (mock). The compiler diagnostics flagged each missing one;
  fix all implementations together when extending an interface.
- **`MockDB` does not implement the full `zhihuDB.DB` interface** (it was already
  missing methods like `GetAnswerAfter`). So the new RSS-dedicated query methods were
  added only to the `DBAnswer` interface + `DBService`; `MockDB` needed no change and
  the build stayed green. Don't assume a type named `Mock*` satisfies the full
  interface — verify via the build instead of pre-emptively padding it.
- **Did not touch the shared queries.** `GetLatestNAnswer` / `GetAnswerAfter` are
  reused by the crawl cursor (`cron/crawl.go`) and statistics
  (`controller/archive/statistics.go`). New `GetLatestNVisibleAnswer` /
  `GetVisibleAnswerAfter` carry the `detect_status <> 2` filter so only RSS is
  affected.
- **Fake AI for tests must match the full interface signature**, e.g. `Text(io.Reader)`
  not `Text(any)`. The detector's only real test seam is `ai.AI`; a fake there drives
  hit / non-hit / malformed-JSON / ai-error / unregistered-author paths with no DB.
- **Fail-open contract**: a malformed JSON reply or a persistent AI error returns
  `detected=true, err!=nil`, which the caller maps to `DetectStatusFailed` (shown, not
  skipped). Only an explicit `{"skip": true}` hides content.

## Decisions carried from SPEC

- Historical backfill is out of scope; existing rows stay `detect_status = 0` (shown).
- `detectCriteria` ships empty (placeholder); the maintainer fills in author criteria.
- No DB integration tests (maintainer decision) — only the detector unit tests.

## Follow-ups

- Maintainer to populate `detectCriteria` in `pkg/routers/zhihu/parse/detect.go`.
- Optional: tune `classifyPromptTmpl` once real criteria land.
