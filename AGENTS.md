# AGENTS.md

Working agreement for humans and agents on RSS-ZERO (the Go backend). Read
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the code map.

## What this is

An all-in-one RSS aggregator that serves private/unfriendly sites (Zhihu, Xiaobot,
ZSXQ, GitHub, tombkeeper, endoflife, macked, …) as Atom, in pure Go — no headless
browser. Content is crawled, parsed, stored (Postgres), and rendered through one
unified `/rss/<source>` pipeline. Deploy/runbook: [docs/OPS.md](docs/OPS.md).

## Related repositories

- `../webapp` — this project's frontend; the two are a paired backend/frontend.
- `../../zhihu-encrypt` — the Zhihu encryption service (`rss-zhihu-encrypt` in compose).

## 开发规则

1. 使用 Conventional Commit 格式，小步提交。
2. 使用 `justfile` 和 `just` 工作流。
3. 开发时从 master 或指定分支创建分支，按变更性质命名（如 `feat/...`、`fix/...`、`build/...`、`chore/...`），完成后 squash merge 到源分支并清理开发分支。
