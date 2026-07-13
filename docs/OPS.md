# 运维手册

构建、发版、部署、迁移、回填、告警。生产机 `linkerlab-us-2`，栈用 docker compose。

## 版本与发布

CalVer `YY.MM.MICRO`（如 `26.7.2`）。server 与 webapp 各自独立一条 CalVer 线，**不要**靠肉眼
猜 creatordate —— 下一个 tag 由 `git tag` 列表经 `sort -V` + calver 推算（justfile 里的
`next_version`），并与 [PROGRESS.md](PROGRESS.md) 对齐。

标准发版走 `/rss-zero-release` skill（在 `server/` 下触发）：算下一个 CalVer → 打并推 git tag →
经 justfile 构建 → 推 `eliyip/rss-zero` 镜像。手动等价物：

```
just build-docker      # scripts/build-docker.sh → eliyip/rss-zero:<tag> + latest
just tpush             # git tag {{next_version}} && git push --tags
```

## 部署（生产）

1. `ssh linkerlab-us-2`
2. 在 `~/services/rss-zero/.env` 里改镜像 tag：`SERVER_TAG`（后端）、`WEBAPP_TAG` /
   `WEBAPP_TEST_TAG`（前端，如同批）。
3. `docker compose pull && docker compose up -d`
4. 核验：`/api/v1/health` 返回目标 version；`rss-db` healthy；启动日志无 error/panic
   （偶见无关的 macked 上游 403 属正常）。

## 运行栈（compose）

- `rss-zero` — 后端，`expose 8080`，日志挂 `/var/log/rss-zero`
- `rss-zhihu-encrypt` — 知乎加密服务，`expose 3000`
- `rss-db` — `postgres:16.3-alpine`，数据挂 `./db-data`，带 `pg_isready` healthcheck
- `rss-redis` — `redis:7-alpine`，数据挂 `./redis-data`

参见 `deploy/compose.yaml` 与 `deploy/config.toml`。

## 配置

`config.toml` 顶层表：`[settings] [minio] [openai] [database] [language_detection]
[redis] [bark] [zlive] [test_url] [utils] [zsxq]`。生产值放部署机的 `deploy/config.toml`，
不进库。

## 迁移

启动时自动跑（`internal/migrate` 注册表的 `RunAuto`）—— 多为「离线回填已存正文」的幂等数据
迁移。失败只记日志并 Bark 通知、下次启动重试，不阻断服务。上线后在启动日志确认迁移已执行
（`scanned N / updated M`）。手动端点：`registry` / `run/:version` / `run-pending`。

## 历史回填（tombkeeper）

`POST /api/v1/tombkeeper/history` 后台跑历史抓取（Asia/Shanghai 日期窗口）。**注意：内存
goroutine，无断点续传，进程重启即静默丢失** —— 长回填要么一次跑完、要么记下中断页码手动重跑。
（例：2010..2025 那次停在 page 2104/5857，需从头重跑。）失败 Bark 通知；进程内单例守卫，
重复触发返回 409。

### 结构化内容迁移（20260711000000）

该自动迁移会清空 `tombkeeper_post`，删除旧的 `title` / `text_markdown` / `video_url` / `raw`
展示缓存列，保留图片资产，再由新模型存结构化内容。它不可由旧版本应用安全回滚；部署前应保留数据库
备份，且不要在迁移后启动旧镜像。

部署顺序：先确认启动日志已记录迁移 `20260711000000`；再调用
`POST /api/v1/job/run/tombkeeper_crawl` 抓最新两页，确认新 Redis source key
`tombkeeper_timeline_rss` 已产生且 RSS 有内容；最后以完整日期范围调用
`POST /api/v1/tombkeeper/history` 恢复历史。history 仍受上述无断点续传限制。

## 知乎 / 星球事实化删列迁移（20260712000000、20260713000000）

知乎与星球改为「只存事实、读取期纯函数渲染」后，随分支带两条**原地删列**自动迁移
（`Auto` + `RequiresPredecessors`）：`20260712000000`（`zsxq-drop-topic-text`）删 `zsxq_topic.text`，
`20260713000000`（`zhihu-drop-content-text`）删 `zhihu_answer` / `zhihu_article` / `zhihu_pin` 的
`text`。二者由框架 `RunAuto` 启动时**自动执行、无手动步骤**；幂等门控为 legacy `text` 列是否仍在，
重试只 no-op。

与 tombkeeper 的重量级迁移不同，**不 TRUNCATE、不重抓、不回填**：正文所需一切已作为事实存在
（`raw` + object/question/article/author 侧表），删列后读取期即可从 `raw` 重放。上线前只读 SQL 审计
确认零丢失——zhihu「`raw` content 空但 `text` 非空」的行数为 **0**（article 0/1794、pin 0/8321、
answer 18/24593，且那 18 行 `text` 本就只有一个 `\n`），zsxq 事实在 `raw` + 侧表同样完整。

**框架不保证的人为残余（需人工留意）**：

- **不可回滚到读 `text` 列的旧二进制**——旧镜像缺列会报错。但 `raw` 未删、源数据不丢，回退手段是
  「补回 `text` 列 + 从 `raw` 重算」或还原 DB 备份，而非靠迁移自动逆转。
- runner 失败非致命（记日志 + Bark、下次启动重试），故**部署后各 smoke-test 一个 RSS**：拉一条
  `/rss/zsxq/*` 与一条 `/rss/zhihu/*`，确认正文从 `raw` 读出正常。
- 两源换新 v2 source cache key，隔离旧 `text` 生成的缓存条目；crawl warm 与 controller cache-miss 同步改 key。
- **已删端点**：`/api/v1/refmt/zhihu`、`/api/v1/refmt/zsxq`、`/api/v1/migrate/20240929`、
  `/api/v1/migrate/20250530` 现均 404/不存在（读取期渲染后「反解 `raw` 重写 `text`」失去意义，旧一次性
  迁移按需删除；xiaobot refmt 仍在）。

## 博客全量抓取（tkblog：xfocus / baidu）

`POST /api/v1/tkblog/:category/crawl`（`category` = `xfocus` | `baidu`，admin 鉴权）后台**全量**抓取
该源全部博文入库（幂等 upsert，每次都是全量）。返回 202 + `job_id`（xid，进日志字段/失败 Bark，供
grep 追踪）；同 category 已在抓取中返回 409。**无 cron、无断点续传**——内容已定型、篇数少，崩了重触发
即可。仅落库 + 单篇归档（`GET /api/v1/archive/<tombkeeper.io/{category}/{id}>`），**无 RSS 出口**。

## 告警

失败路径统一 Bark 推送（迁移失败、回填失败等）。健康检查 `/api/v1/health` 带 version，用于部署后核验。
