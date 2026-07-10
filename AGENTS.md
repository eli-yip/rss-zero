# AGENTS.md

Working agreement for humans and agents on RSS-ZERO (the Go backend). Read
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the code map and
[docs/CONVENTIONS.md](docs/CONVENTIONS.md) for how we track work.

## What this is

An all-in-one RSS aggregator that serves private/unfriendly sites (Zhihu, Xiaobot,
ZSXQ, GitHub, tombkeeper, endoflife, macked, …) as Atom, in pure Go — no headless
browser. Content is crawled, parsed, stored (Postgres), and rendered through one
unified `/rss/<source>` pipeline. Deploy/runbook: [docs/OPS.md](docs/OPS.md).

## Related repositories

- `../webapp` — this project's frontend; the two are a paired backend/frontend.
- `../../zhihu-encrypt` — the Zhihu encryption service (`rss-zhihu-encrypt` in compose).

## Rules

1. **Language.** Docs and comments in **Chinese**; commit messages in **English**
   (Conventional Commits). Before writing a commit message, read the recent commit
   history and match its style.

2. **Docs.** Keep a light paper trail — [docs/CONVENTIONS.md](docs/CONVENTIONS.md) is
   the full convention (flat dirs, frontmatter status, `YYYY-MM-DD-slug` names, the
   issue/plan/lesson schema):
   - **Reference docs** (evergreen, maintained in place):
     [ARCHITECTURE](docs/ARCHITECTURE.md) · [OPS](docs/OPS.md) ·
     [PROGRESS](docs/PROGRESS.md) · [TODO](docs/TODO.md).
   - **Workflow docs**: `docs/issues/` (one problem/task/bug) · `docs/plans/` (one
     approach; **where decisions are recorded**) · `docs/lessons/` (execution retro).
   - Find work by status: `just issues open` / `just plans in-progress` /
     `just lessons draft` (also `closed`, `wontfix`, `done`).

3. **The flow.** issue (what & why) → **plan before any code** → implement, recording a
   lesson → follow-up issues as they surface. Every issue gets a plan first; no issue
   goes straight to implementation. Commit each issue/plan as its own `docs(...)` commit
   before writing code. Update [docs/PROGRESS.md](docs/PROGRESS.md) in the **same commit**
   as the doc change that finishes a branch — never later. Anything outside the current
   plan (deferred fixes, tech debt, ideas) goes in [docs/TODO.md](docs/TODO.md), not into
   the current change.

4. **Two independent reviews per plan — reviewer ≠ author.** (1) _Plan review_ before
   building: a fresh read against the issue. (2) _Implementation review_ before merging:
   run `/code-review` (Standards + Spec) on the diff; findings are fixed or spun out as
   follow-up issues, never a standalone file.

5. **Tests are not optional.** Every plan adds tests; every fixed bug gets a regression
   test that fails before the fix. RSS sources keep golden snapshots (`testdata/*.atom`).
   Unless explicitly asked, do **not** run the full suite — prefer the touched packages'
   tests (`go test ./internal/rss/...`).

6. **Lint.** Run `just lint`; auto-fix with `just fix-lint`. Do **not** write your own
   lint command — the recipe chains autocorrect / dprint / go mod tidy / golangci-lint /
   go fix in the right order.

7. **Git flow.** Branch off `master` as `feat-…` / `fix-…` / `chore-…`, small focused
   commits. When done: update PROGRESS, get the review, then **squash-merge** into
   `master` and **delete** the branch.

## Common commands

- `just server` / `just cli` — run the server / CLI
- `just lint` / `just fix-lint` — lint (report) / auto-fix
- `just test` — `go test` in the current dir; scope by package path for a targeted run
- `just build` / `just build-docker` — build binary / Docker image
- `just issues [status]` / `just plans [status]` / `just lessons [status]` — list docs by status
- Release: `/rss-zero-release` skill (CalVer tag → build → push `eliyip/rss-zero`); deploy per [docs/OPS.md](docs/OPS.md)
