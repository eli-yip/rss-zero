# SPEC: Unified Cookie Update Interface

- Date: 2026-06-11
- Status: Accepted
- Scope: server cookie update/consume/check paths + `cookie-updater` browser extension

## 1. Problem

The current cookie system treats **each cookie** as **its own backend feature**:

- Four per-platform controllers (`controller/{zsxq,zhihu,xiaobot,github}`), each with a
  bespoke DTO and handler. Zhihu's `UpdateCookie` is ~150 lines of three near-identical
  blocks.
- Four request shapes that disagree (`access_token:{value,expire_at}` vs three named
  cookies vs `{token}` vs `{token,expire_at:"2006-01-02"}`). `expire_at` is variously a
  JS date string, a Unix float, or a date.
- Four consumer load helpers that disagree in verbosity and in missing/invalid handling
  (zhihu deletes+notifies+stops; github never cleans up an invalid token).
- Adding a platform/cookie requires: a new `CookieTypeXxx` const, a `TypeToStr` arm, a new
  controller + DTO, a new route, and bespoke expire parsing.

Difference between platforms is **data** (domain, cookie name, safety gap, optional probe),
not **code structure**. This SPEC collapses that data into one registry and unifies all
three paths (write / consume / check) around it.

## 2. Goal

1. **Write**: one generic endpoint accepting the browser's native cookie objects.
2. **Consume**: two shared helpers (`Bundle`, `Invalidate`) replace the four bespoke flows.
3. **Check**: one `GET` dashboard endpoint + a registry-driven daily cron.
4. **Extension**: dump all cookies for the active domain and POST them; no per-platform code.
   The popup then shows **which cookies were stored and their expiry**, parsed from the
   endpoint's per-cookie response (today the popup only says "刷新成功" with no detail).

## 3. Decisions (agreed)

- **Storage stays `int`.** The existing `CookieTypeXxx` iota constants and the `cookies.type`
  column are unchanged — **zero DB migration**, consumers keep calling `cookie.Get(TypeXxx)`.
  The registry is a `name→Type` translation + policy layer, not a new storage key.
- **Extension host permissions: per-domain allowlist.** Adding a platform edits the manifest
  `host_permissions` + rebuilds. Accepted (self-distributed, unpacked extension).
- **xiaobot / github stay manual for now**, but POST to the same generic endpoint (github PAT
  is hand-typed with an explicit expiry; xiaobot token source — cookie vs localStorage — is
  verified during implementation, see §9).
- **Old `/cookie/{platform}` endpoints are kept during transition**, removed only after the
  new extension is verified (§8 P3). New and old coexist; nothing breaks mid-migration.

## 4. Registry — the single source of truth

New file `pkg/cookie/registry.go`. One `Spec` per cookie; everything platform-specific lives
here and nowhere else.

```go
type Spec struct {
    Type       int           // existing CookieTypeXxx (storage key, unchanged)
    Platform   string        // "zhihu"
    Name       string        // browser cookie name; match key on write
    Domains    []string      // [".zhihu.com"]; match key on write
    SafetyGap  time.Duration // subtract from real expiry: zse_ck 48h, d_c0/z_c0 24h
    DefaultTTL time.Duration // used when the cookie carries no expirationDate
    Manual     bool          // true = not browser-grabbed (github PAT)
    Probe      Probe         // optional validator (zsxq/xiaobot); nil = no probe
}

type Probe func(value string, l *zap.Logger) error

var registry = []Spec{
  {Type: CookieTypeZhihuDC0,   Platform: "zhihu", Name: "d_c0",     Domains: []string{".zhihu.com"}, SafetyGap: 24 * time.Hour},
  {Type: CookieTypeZhihuZC0,   Platform: "zhihu", Name: "z_c0",     Domains: []string{".zhihu.com"}, SafetyGap: 24 * time.Hour},
  {Type: CookieTypeZhihuZSECK, Platform: "zhihu", Name: "__zse_ck", Domains: []string{".zhihu.com"}, SafetyGap: 48 * time.Hour},
  {Type: CookieTypeZsxqAccessToken,    Platform: "zsxq",    Name: "zsxq_access_token", Domains: []string{".zsxq.com"}},
  {Type: CookieTypeXiaobotAccessToken, Platform: "xiaobot", Name: "token",             Domains: []string{".xiaobot.net"}},
  {Type: CookieTypeGitHubAccessToken,  Platform: "github",  Name: "github",            Manual: true},
}
```

**Probes are registered at startup, not literal in the registry — this avoids an import
cycle.** `zsxqProbe`/`xiaobotProbe` call `pkg/routers/{zsxq,xiaobot}/request`, which in turn
import `pkg/cookie`; embedding them here would make `pkg/cookie` import those packages back.
Instead `pkg/cookie` exposes `func RegisterProbe(t int, p Probe)` that fills `Spec.Probe`, and
the server wiring (which already imports both `cookie` and the request packages) registers them
during startup. The xiaobot probe needs the `CookieIface`; the wiring closure captures it:

```go
cookie.RegisterProbe(cookie.CookieTypeZsxqAccessToken, func(v string, l *zap.Logger) error {
    _, err := zsxqreq.NewRequestService(v, l).Limit(context.Background(), config.C.TestURL.Zsxq, l)
    return err
})
cookie.RegisterProbe(cookie.CookieTypeXiaobotAccessToken, func(v string, l *zap.Logger) error {
    _, err := xiaobotreq.NewRequestService(cs, v, l).Limit(config.C.TestURL.Xiaobot)
    return err
})
```

Lookup helpers (the only API other layers use):

```go
func specByNameDomain(name, domain string) (Spec, bool) // write path; domain optional
func specsByPlatform(platform string) []Spec            // consume path (zhihu → 3 specs)
func specByType(t int) (Spec, bool)                     // messaging / TypeToStr
func allSpecs() []Spec                                  // check path
```

`TypeToStr` is rewritten to derive from the registry (`Platform + "_" + Name`), removing the
hand-written switch.

## 5. Write path — one generic endpoint

Registered on the existing `cookieApi := apiGroup.Group("/cookie")` group root (already in
`groupNeedAuth`, so it inherits `AllowAdmin` for free — no new group/middleware wiring):

```
POST /api/v1/cookie           (admin-only, group root)
{
  "cookies": [
    { "name": "d_c0", "value": "<raw>", "expirationDate": 1722069482.1, "domain": ".zhihu.com" },
    ...
  ]
}
```

Body is the browser's native cookie shape (`chrome.cookies.Cookie`): `name`, `value`,
optional `expirationDate` (Unix seconds, float), optional `domain`. New controller
`controller/cookie` (one file). Per incoming cookie:

1. `specByNameDomain(name, domain)`; unmatched → record `{stored:false, reason:"not registered"}`, skip.
2. `value = ExtractCookieValue(value, name)` (strip a `name=` prefix if present); empty → `reason:"empty value"`.
3. `ttl`: if `expirationDate` present → `time.Until(expiry) - Spec.SafetyGap`; else `Spec.DefaultTTL`.
   Non-positive ttl → `reason:"already expired"`, skip.
4. If `Spec.Probe != nil` → run it; failure → `reason:"validation failed: ..."`, skip.
5. `cookie.Set(Spec.Type, value, ttl)`.

Response (per-cookie, so the extension popup / dashboard can show outcomes). `expire_at` is
RFC3339; the popup renders the stored cookies as `name — <local time>` and lists/omits the
ignored ones:

```json
{ "results": [ {"name":"d_c0","platform":"zhihu","stored":true,"expire_at":"2026-07-26T16:48:02+08:00"},
               {"name":"_ga","stored":false,"reason":"not registered"} ] }
```

Only cookies present in the payload are touched; unmentioned cookies are left intact (no
accidental deletion). `expirationDate` parsing reuses the existing `ParseArcExpireAt` (already
handles the `float64` Unix branch and the legacy Arc string).

## 6. Consume path — two shared helpers

New helpers in `pkg/cookie` collapse the four bespoke load/invalidate flows. Detection of an
invalid cookie stays platform-specific (different HTTP codes); the **policy** (delete + notify)
and the **load + missing-notify** become shared.

```go
// Bundle loads every cookie a platform needs. On the first missing one it notifies the user
// with a uniform message derived from the registry and returns a sentinel error, so the caller
// just `if err != nil { return }`. Keys are cookie names.
func Bundle(cs CookieIface, platform string, n notify.Notifier, l *zap.Logger) (map[string]string, error)

// Invalidate deletes a cookie and notifies the user that <platform>/<name> must be refreshed.
// Replaces removeZC0Cookie / handleInvalidZsxqCookie / the in-service Del calls.
func Invalidate(cs CookieIface, cookieType int, n notify.Notifier, l *zap.Logger)
```

Caller examples:

```go
// zhihu: replaces GetZhihuCookies (47 lines) + HandleZhihuCookiesErr + 3 error types
b, err := cookie.Bundle(cs, "zhihu", notifier, logger)
if err != nil { return } // already notified which cookie is missing
zc := &request.Cookie{DC0: b["d_c0"], ZC0: b["z_c0"], ZseCk: b["__zse_ck"]}

// on upstream rejection (still detected per-platform):
cookie.Invalidate(cs, cookie.CookieTypeZhihuZSECK, notifier, logger)
```

zsxq / xiaobot / github single-cookie loaders become a one-line `Bundle(cs, "<platform>", …)`
+ map read. github gains the invalid-token cleanup it currently lacks.

## 7. Check path — one dashboard endpoint

```
GET /api/v1/cookie    (admin-only, group root)
{ "cookies": [ {"platform":"zhihu","name":"d_c0","manual":false,"stored":true,
                "expire_at":"...","healthy":true}, ... ] }
```

Iterates `allSpecs()`; `healthy = stored && CheckTTL(Type, 48h) == nil`. Replaces the four
`CheckCookie` handlers. `checkCookies` cron (`cmd/server/cron.go`) iterates `allSpecs()`
instead of `GetCookieTypes()`, so a never-set cookie is also flagged (today's cron only warns
about already-present-but-expiring cookies).

## 8. Migration order (nothing breaks at any point)

| Phase | Action | Risk |
| --- | --- | --- |
| P1 server | Add registry + `POST/GET /api/v1/cookies` + `Bundle`/`Invalidate`; refactor the four consumers. **Keep old `/cookie/{platform}` endpoints.** | none — old+new coexist |
| P2 extension | Switch to `chrome.cookies.getAll({domain})` + POST the generic endpoint; install & verify. | none — old endpoints still back it |
| P3 cleanup | After the new path is confirmed live, delete the four old controllers + DTOs + routes. | low |

## 9. Open items

- **xiaobot token source.** Verify during P1/P2 whether the xiaobot bearer token lives in a
  cookie named `token` on `.xiaobot.net` or in `localStorage`. If localStorage, the extension
  needs a content script; until then xiaobot stays a manual POST to the generic endpoint.
- **zsxq/xiaobot probes.** `zsxqProbe`/`xiaobotProbe` port the existing async test-request
  validation out of the old controllers into the registry. zsxq's old flow is async with a
  `request_id`; the new generic endpoint runs the probe synchronously (admin call, low volume).
- **Extension auth & config.** Basic-auth credentials and base URL stay hardcoded as today; not
  in scope. (Flagged for a later pass.)
