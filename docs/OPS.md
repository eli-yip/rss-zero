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

## 告警

失败路径统一 Bark 推送（迁移失败、回填失败等）。健康检查 `/api/v1/health` 带 version，用于部署后核验。
