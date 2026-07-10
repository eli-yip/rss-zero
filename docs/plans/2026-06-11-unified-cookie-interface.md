# PLAN: Unified Cookie Update Interface

- Date: 2026-06-11
- SPEC: [issues/2026-06-11-unified-cookie-interface.md](../issues/2026-06-11-unified-cookie-interface.md)
- Branch: `feat/unified-cookie-interface`

## Recon summary (verified against current code)

- **Route group**: `cmd/server/echo.go:139` `cookieApi := apiGroup.Group("/cookie")`, appended
  to `groupNeedAuth` (`:140`) which all receive `AllowAdmin()` (`:171`). Adding `POST /` and
  `GET /` to `cookieApi` yields admin-protected `POST/GET /api/v1/cookie` with zero new wiring.
- **Storage unchanged**: `pkg/cookie/impl.go` `Set/Get/Del/CheckTTL/GetTTL` keyed by `int`
  `type`; `cookies` table auto-migrated in `internal/migrate/db.go`. No schema change.
- **Import-cycle constraint**: `pkg/routers/xiaobot/request` imports `pkg/cookie`
  (`NewRequestService(cs cookie.CookieIface, …)`); `pkg/routers/zsxq/request` is imported by the
  controller alongside `cookie`. Probe closures must therefore be **registered from the wiring
  layer**, not embedded in `pkg/cookie`. SPEC §4.
- **Probe primitives that exist**:
  - zsxq: `zsxqreq.NewRequestService(token, logger).Limit(ctx, config.C.TestURL.Zsxq, logger)`
    (`controller/zsxq/cookie.go:105,114`).
  - xiaobot: `xiaobotreq.NewRequestService(cs, token, logger).Limit(config.C.TestURL.Xiaobot)`
    (`controller/xiaobot/token.go:35`).
- **Consumers to refactor**:
  - zhihu: `pkg/cookie/zhihu.go` `GetZhihuCookies` (47L) + `HandleZhihuCookiesErr`;
    `pkg/routers/zhihu/cron/crawl_err.go` `removeZC0Cookie`/`removeZSECKCookie`.
  - zsxq: `pkg/routers/zsxq/cron/crawl.go` `getZsxqCookie` (`:279`),
    `handleInvalidZsxqCookie` (`:242`).
  - xiaobot: `pkg/routers/xiaobot/cron/cron.go` `getXiaobotToken` (`:95`); in-service `Del`
    at `pkg/routers/xiaobot/request/request.go` (`validateAPIResp`).
  - github: inline `Get` in `pkg/routers/github/cron/crawl.go:44`; no invalid cleanup today.
- **Daily check**: `cmd/server/cron.go:200` `checkCookies` iterates `GetCookieTypes()`
  (only already-stored types). Switch to registry so never-set cookies are flagged too.
- **expire parsing**: reuse `pkg/cookie/parse.go` `ParseArcExpireAt` (float64 Unix +
  legacy Arc string) and `ExtractCookieValue`.

## Server steps (Phase P1)

Each step: files, change, verify. Ordered by dependency. Commit per step.

### Step 1 — Registry + probe registration (new file)

- File: `pkg/cookie/registry.go` (new).
- Add `Spec` struct, `Probe` type, `registry []Spec`, and a private `probes map[int]Probe`
  populated by `RegisterProbe(t int, p Probe)` (Spec resolves its probe via this map at call
  time, so order of init vs registration does not matter).
- Lookup helpers: `specByNameDomain(name, domain string) (Spec, bool)` (match `Name`, and if
  `domain != ""` require it to hint-match one of `Domains` via suffix; domain is advisory, not
  required), `SpecsByPlatform(platform string) []Spec`, `SpecByType(t int) (Spec, bool)`,
  `AllSpecs() []Spec`, `ProbeFor(t int) Probe`.
- Rewrite `TypeToStr` (`pkg/cookie/iface.go`) to derive from `SpecByType` (`Platform_Name`),
  falling back to `"unknown"`.
- Verify: `go build ./...`.

### Step 2 — Consume helpers: `Bundle` + `Invalidate` (new file)

- File: `pkg/cookie/consume.go` (new). Imports `notify`, `zap` (already used in this pkg).
- `Bundle(cs CookieIface, platform string, n notify.Notifier, l *zap.Logger) (map[string]string, error)`:
  for each `SpecsByPlatform(platform)`, `cs.Get(spec.Type)`; on `ErrKeyNotExist` (or empty),
  `notify` "Need to update <platform>/<name> cookie" and return a sentinel `ErrCookieMissing`
  (wrapped with the name). Other errors returned as-is. Success → `map[name]value`.
- `Invalidate(cs CookieIface, t int, n notify.Notifier, l *zap.Logger)`: `cs.Del(t)` + notify
  "<platform>/<name> invalid, please refresh" using `SpecByType`. Logs on Del failure.
- Verify: `go build ./...`; small table test `consume_test.go` with a fake `CookieIface`
  (missing → notified+sentinel; present → map).

### Step 3 — Generic write + check controller (new package)

- File: `internal/controller/cookie/cookie.go` (new package `cookie` under controller). Holds a
  `Controller{ cookie cookie.CookieIface }` + constructor, mirroring existing controllers.
- `UpdateCookies(c echo.Context)` — `POST /api/v1/cookie`:
  - Bind `{ cookies: []InCookie }`, `InCookie{ Name, Value string; ExpirationDate *float64;
    Domain string }`.
  - Per cookie: `specByNameDomain` → if none, result `{stored:false, reason:"not registered"}`.
    `ExtractCookieValue(value, name)`; empty → `reason:"empty value"`. ttl from
    `ExpirationDate` (`time.Until(unix) - SafetyGap`) else `DefaultTTL`; `<=0` →
    `reason:"already expired"`. If `ProbeFor(type) != nil` run it; err → `reason:"validation
    failed: …"`. Else `cookie.Set(type, value, ttl)`; result `{stored:true, expire_at}`.
  - Respond `{ results: []Result }`.
- `CheckCookies(c echo.Context)` — `GET /api/v1/cookie`: iterate `AllSpecs()`;
  `stored = Get != ErrKeyNotExist`; `expire_at` via `GetTTL`; `healthy = stored &&
  CheckTTL(type, 48h) == nil`. Respond `{ cookies: []Status }`.
- Verify: `go build ./...`.

### Step 4 — Wire routes + probe registration

- File: `cmd/server/echo.go` — in `registerCookie`, add
  `apiGroup.POST("", cookieHandler.UpdateCookies)` and `apiGroup.GET("", cookieHandler.CheckCookies)`
  (named routes). Thread the new `*cookieController.Controller` through `registerCookie`'s
  signature and its caller at `:141` (construct it where the other handlers are built).
- File: wiring (where services/handlers are constructed in `cmd/server`) — call
  `cookie.RegisterProbe(...)` for zsxq + xiaobot with closures capturing `cs`
  (SPEC §4 snippet). Keep github probe-less.
- Verify: `go build ./...`; boot locally, `curl` the new `GET /api/v1/cookie` (admin header /
  debug mode) returns all six specs; `POST` a zhihu cookie payload stores it.

### Step 5 — Refactor zhihu consumer

- File: `pkg/cookie/zhihu.go` — replace `GetZhihuCookies` body with `Bundle(cs,"zhihu",…)` +
  assemble `request.Cookie{DC0,ZC0,ZseCk}` from the map. Keep the exported signature stable
  (`(*request.Cookie, error)`) so `initZhihuServices` is untouched, OR inline at the call site
  and delete the helper — pick whichever keeps the diff smallest; prefer keeping signature.
- Drop `HandleZhihuCookiesErr`’s three-way switch in favor of the sentinel from `Bundle`
  (caller already notified) — update `pkg/routers/zhihu/cron/crawl.go:52-60` accordingly.
- File: `pkg/routers/zhihu/cron/crawl_err.go` — replace `removeZC0Cookie`/`removeZSECKCookie`
  - notify pairs with `cookie.Invalidate(cs, type, notifier, logger)`.
- Verify: `go build ./...`; `go test ./pkg/routers/zhihu/...` (targeted, if cheap).

### Step 6 — Refactor zsxq / xiaobot / github consumers

- zsxq `pkg/routers/zsxq/cron/crawl.go`: `getZsxqCookie` → `Bundle(cs,"zsxq",…)["zsxq_access_token"]`;
  `handleInvalidZsxqCookie` → `cookie.Invalidate(cs, CookieTypeZsxqAccessToken, …)`.
- xiaobot `pkg/routers/xiaobot/cron/cron.go`: `getXiaobotToken` → `Bundle(cs,"xiaobot",…)`.
  In `request/request.go validateAPIResp`, replace the raw `cs.Del` with `cookie.Invalidate`
  (adds the missing user notification).
- github `pkg/routers/github/cron/crawl.go`: load via `Bundle(cs,"github",…)`; on a release
  request 401/403, call `cookie.Invalidate(cs, CookieTypeGitHubAccessToken, …)` — closes the
  current "stale token never cleaned" gap (SPEC §1).
- Verify: `go build ./...`.

### Step 7 — Registry-driven daily check

- File: `cmd/server/cron.go` `checkCookies` — iterate `cookie.AllSpecs()` instead of
  `GetCookieTypes()`; for each, `CheckTTL(type, 48h)`; `ErrKeyNotExist` → notify
  "Need to update <platform>/<name>". Manual specs (github) included.
- Verify: `go build ./...`.

### Step 8 — Docs bookkeeping

- Update `docs/PROGRESS.md` row as steps land.
- Maintain `docs/lessons/2026-06-11-unified-cookie-interface.md`: append while executing,
  reorganize into a summary at the end (AGENTS.md).

## Extension steps (Phase P2 — repo `/Users/yip/new_home/projects/cookie-updater`)

### Step 9 — Generic POST from full cookie dump

- `src/background.ts`: replace per-platform `getCookie`+body assembly with
  `chrome.cookies.getAll({ domain })` for the active tab's domain → POST the raw array as
  `{ cookies: [...] }` to `${API_ENDPOINT}` (now the bare `/api/v1/cookie`, drop the
  `/zhihu`,`/zsxq` suffixes). Keep Basic-auth header as-is.
- **Popup detail (new requirement)**: parse the endpoint's `results` and render the stored
  cookies as a list of `name — <expiry>`, with `expire_at` formatted to local time (e.g.
  `toLocaleString`). Ignored cookies (`stored:false`) are summarized or omitted, not shown as
  failures. Replaces the current detail-less "刷新成功". `popup.ts` needs the background
  message to carry the `results` array back (today it only relays a `message` string), and
  `popup.html` gets a small list container.
- `public/manifest.json`: keep per-domain `host_permissions` (decision §3); already covers
  zhihu + zsxq. Adding a platform later = add its domain here + rebuild.
- Verify: `bun run build`; load unpacked; on zhihu.com click → popup lists `d_c0`, `z_c0`,
  `__zse_ck` each with an expiry; on zsxq.com → `zsxq_access_token` with expiry; cross-check
  against `GET /api/v1/cookie` and server logs.

## Cleanup (Phase P3 — after P2 verified)

### Step 10 — Remove legacy per-platform cookie endpoints

- Delete `controller/{zsxq/cookie.go, zhihu/cookie.go, xiaobot/token.go, github/token.go}`
  handlers + their routes in `registerCookie`; drop now-unused DTOs. Keep the shared
  `controller/common.Cookie` only if still referenced.
- Verify: `go build ./...`; grep for removed handler names returns nothing.

## Out of scope (SPEC §9)

- Moving cookie storage off `int` keys.
- Extension auth/base-URL config (stays hardcoded).
- xiaobot localStorage content-script (only if the token turns out not to be a cookie).

## Verification at the end

- `go build ./...` clean.
- Targeted tests: `go test ./pkg/cookie/...` (Bundle/Invalidate + any registry test).
- Manual: `GET /api/v1/cookie` lists six specs with health; `POST` stores; extension round-trips
  zhihu + zsxq; daily-check cron message wording sane.
