# PROGRESS

Tracks current work across SPECs, PLANs, and their execution.

| Date | Topic | SPEC | PLAN | Status |
| --- | --- | --- | --- | --- |
| 2026-06-11 | zhihu-content-detection | [specs/2026-06-11-01-zhihu-content-detection.md](specs/2026-06-11-01-zhihu-content-detection.md) | [plans/2026-06-11-01-zhihu-content-detection.md](plans/2026-06-11-01-zhihu-content-detection.md) | Done; detectCriteria populated; merged to master |
| 2026-06-11 | unified-cookie-interface | [specs/2026-06-11-02-unified-cookie-interface.md](specs/2026-06-11-02-unified-cookie-interface.md) | [plans/2026-06-11-02-unified-cookie-interface.md](plans/2026-06-11-02-unified-cookie-interface.md) | Done. P1 server + P2 extension + P3 legacy removal; zhihu & zsxq verified live against local server |
| 2026-06-12 | zsxq-config-externalization | [specs/2026-06-12-02-zsxq-config-externalization.md](specs/2026-06-12-02-zsxq-config-externalization.md) | [plans/2026-06-12-04-zsxq-config-externalization.md](plans/2026-06-12-04-zsxq-config-externalization.md) | **Done & 已上线。** 4 项合并 master（4e939a0）+ lint 修复（424d7db）。发版 26.6.10，现网部署成功（health ok，version 26.6.10）。作者屏蔽已读 [zsxq] 配置生效 |
| 2026-06-12 | zsxq-maintainability-refactor | [specs/2026-06-12-01-zsxq-maintainability-refactor.md](specs/2026-06-12-01-zsxq-maintainability-refactor.md) | 项二 [plans/2026-06-12-01-zsxq-crawl-dead-code.md](plans/2026-06-12-01-zsxq-crawl-dead-code.md) · 项一 [plans/2026-06-12-02-zsxq-request-retry.md](plans/2026-06-12-02-zsxq-request-retry.md) · 项三 [plans/2026-06-12-03-zsxq-object-uri.md](plans/2026-06-12-03-zsxq-object-uri.md) | **Done.** 三项全部合并 master：项二删死代码（a9a58f4）、项一重试收敛（507f504）、项三 URI 收敛（3fe71d0）。附录"配置即代码"记为后续项 |
| 2026-06-13 | lint-modernization | — | — | **工具接入完成。** `just lint` 对齐 maestro-engine（新增 autocorrect / dprint / go mod tidy -diff / go fix），存量问题不修，记入 [TODO.md](TODO.md) 待清理 |
