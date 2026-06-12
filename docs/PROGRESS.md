# PROGRESS

Tracks current work across SPECs, PLANs, and their execution.

| Date | Topic | SPEC | PLAN | Status |
| --- | --- | --- | --- | --- |
| 2026-06-11 | zhihu-content-detection | [specs/2026-06-11-01-zhihu-content-detection.md](specs/2026-06-11-01-zhihu-content-detection.md) | [plans/2026-06-11-01-zhihu-content-detection.md](plans/2026-06-11-01-zhihu-content-detection.md) | Done; detectCriteria populated; merged to master |
| 2026-06-11 | unified-cookie-interface | [specs/2026-06-11-02-unified-cookie-interface.md](specs/2026-06-11-02-unified-cookie-interface.md) | [plans/2026-06-11-02-unified-cookie-interface.md](plans/2026-06-11-02-unified-cookie-interface.md) | Done. P1 server + P2 extension + P3 legacy removal; zhihu & zsxq verified live against local server |
| 2026-06-12 | zsxq-config-externalization | [specs/2026-06-12-02-zsxq-config-externalization.md](specs/2026-06-12-02-zsxq-config-externalization.md) | [plans/2026-06-12-04-zsxq-config-externalization.md](plans/2026-06-12-04-zsxq-config-externalization.md) | 已实现（分支 feat-zsxq-config，4 项：作者屏蔽进 [zsxq] 配置 + 业务码/魔法日期/topicIDSkip 提常量/map）。现网 config.toml 已补 [zsxq]（备份 .bak-20260612），待 review。项1 生效需后续重新部署镜像 |
| 2026-06-12 | zsxq-maintainability-refactor | [specs/2026-06-12-01-zsxq-maintainability-refactor.md](specs/2026-06-12-01-zsxq-maintainability-refactor.md) | 项二 [plans/2026-06-12-01-zsxq-crawl-dead-code.md](plans/2026-06-12-01-zsxq-crawl-dead-code.md) · 项一 [plans/2026-06-12-02-zsxq-request-retry.md](plans/2026-06-12-02-zsxq-request-retry.md) · 项三 [plans/2026-06-12-03-zsxq-object-uri.md](plans/2026-06-12-03-zsxq-object-uri.md) | **Done.** 三项全部合并 master：项二删死代码（a9a58f4）、项一重试收敛（507f504）、项三 URI 收敛（3fe71d0）。附录"配置即代码"记为后续项 |
