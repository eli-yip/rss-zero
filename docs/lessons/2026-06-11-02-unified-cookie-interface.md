# LESSON: Unified Cookie Update Interface

- Date: 2026-06-11
- SPEC: [specs/2026-06-11-02-unified-cookie-interface.md](../specs/2026-06-11-02-unified-cookie-interface.md)
- PLAN: [plans/2026-06-11-02-unified-cookie-interface.md](../plans/2026-06-11-02-unified-cookie-interface.md)

> Append notes while executing each step; reorganize into a coherent summary once the PLAN
> completes (per AGENTS.md).

## Notes (chronological)

- (pre-impl) Probes cannot be embedded as literals in the `pkg/cookie` registry: the platform
  request packages import `pkg/cookie`, so a literal probe would create an import cycle. Resolved
  by registering probe closures from the wiring layer at startup (`RegisterProbe`).
- (pre-impl) The new generic endpoints reuse the existing `/api/v1/cookie` group root, which is
  already in `groupNeedAuth` → inherits `AllowAdmin` with no extra middleware wiring.
- (Step 5) `GetZhihuCookies` has 5 callers — two are RSS-read / parse-controller paths and one is
  a request test, where a push notification would be _wrong_. So `Bundle` (which notifies) is the
  right tool for the single-cookie cron consumers but not for zhihu's loader. Kept
  `GetZhihuCookies` non-notifying with a stable signature (collapsed via the registry, returns
  `ErrCookieMissing`); only the cron path notifies, via the simplified `HandleZhihuCookiesErr`.
- (Step 6) zsxq's old `getZsxqCookie` deleted an _empty_ cookie row. Dropped that: the new write
  path never stores empty values, and deleting during a read is a side effect best avoided. Legacy
  empty rows just keep getting flagged by Bundle until re-set.
- (Step 6) xiaobot request service has no notifier and threading one in would touch 3 call sites
  (controller, cron, probe). Left `request.go` untouched (it already deletes the bad token) and
  added the user-facing notification at the cron layer via `cookie.Invalidate` on `ErrNeedLogin`.
  The extra `Del` inside `Invalidate` is a harmless idempotent no-op.
- (Step 6) github invalid-token cleanup keys on HTTP **401 only** (`ErrUnauthorized`). 403 is
  excluded because GitHub also returns it for rate limiting, where the token is still valid —
  invalidating then would wrongly delete a good token.
- (Step 8) `new(expr)` (Go 1.26) replaces a `ptr()` test helper for `*float64` literals.

## Summary (P1 — server)

Done: registry (`pkg/cookie/registry.go`) is the single source of truth; `Bundle`/`Invalidate`
(`consume.go`) collapse the four bespoke load/invalidate flows; generic `POST/GET /api/v1/cookie`
(`internal/controller/cookie`) replace the per-platform update/check handlers (legacy still live);
all four consumers + the daily check refactored onto the registry. Build, `go vet`, and the new
unit tests (`pkg/cookie`, `internal/controller/cookie`) are green. Legacy `/cookie/{platform}`
endpoints intentionally remain until P3.

Not yet verified live (no local DB/redis boot in this pass): the running endpoints and the daily
cron wording. To verify alongside P2 (extension) against a real server.

## Summary (P2 — extension)

Done (repo `cookie-updater`, branch `feat/generic-cookie-endpoint`, v1.3.0): the popup requests
host access for the active site, reads every cookie via `chrome.cookies.getAll`, and POSTs them to
the generic `POST /api/v1/cookie`; it renders each stored cookie's name + local expiry. All work
runs in the popup's user-gesture context; the service worker is a stub. All per-platform request
assembly deleted. Compiled `public/*.js` is gitignored (rebuilt on load); only `manifest.json` +
`popup.html` are tracked.

### Chrome cookies-API permission model — the hard-won lesson

Reading cookies took several iterations because Chrome gates `cookies.getAll` by whether the
extension holds a host-permission **match pattern** that covers the cookie — on BOTH axes:

1. **Permission model.** Static `host_permissions` for specific sites were not effective at runtime
   in Arc (couldn't toggle "on all sites"; `getAll` returned 0). Fix: `optional_host_permissions:
   ["<all_urls>"]` + `chrome.permissions.request` on click (cookie-editor's model).
2. **Domain axis.** Requesting only the exact host `https://www.zhihu.com/*` misses **parent-domain**
   cookies (`.zhihu.com`: d_c0/z_c0/__zse_ck). Must also request the wildcard subdomain `*.zhihu.com/*`.
3. **Scheme axis.** With `https://*.zhihu.com/*`, only `secure:true` cookies came through; `d_c0`
   (`secure:false`) was dropped. Chromium maps a non-secure cookie to an **`http://`** permission
   URL, so an https-only grant filters every non-secure cookie. Fix: scheme wildcard `*://`.

Working recipe (mirrors `Moustachauve/cookie-editor` `interface/lib/permissionHandler.js`):
`request({ origins: ["*://<host>/*", "*://*.<rootDomain>/*"] })`, then
`cookies.getAll({ url: tab.url, storeId: tab.cookieStoreId })`. Red herring along the way: Arc's
`tab.cookieStoreId` is `undefined` (default store is correct), so the store was never the problem.

## P3 — done

Legacy `/cookie/{platform}` controllers + routes removed, plus the orphaned `common.Cookie` DTO and
the now-unused `ParseArcExpireAt`. Verified live: generic `GET /api/v1/cookie` lists all six specs;
legacy paths return 404. zhihu (3 cookies) and zsxq (probe) both stored end-to-end via the extension.
