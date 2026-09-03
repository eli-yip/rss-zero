---
title: "微博图片下载将 CDN 欢迎页误存为图片"
kind: bug
status: open
priority: high
areas: [tombkeeper, migrate]
plan: docs/plans/2026-09-03-tombkeeper-image-validation.md
updated: "2026-09-03"
---

生产微博 `https://weibo.com/1401527553/Rg3CcBv1Y` 的四张关联图片中，三张 OSS 文件实际为
4,496 字节 HTML，标题为 `Welcome to Rainbow IPFS Gateway!`，但扩展名、存储类型和数据库状态
分别为 `.jpg`、`image/jpeg`、成功。三张记录均来自 `cdn.ipfsscan.io`，其当前响应与错误文件哈希一致。
生产库只读统计发现该 CDN 的成功资产记录共 1,743 条；该数量是待排查范围，不是已确认损坏数量。

下载入口只检查 HTTP 200 和 Content-Length 非零，HTML 能在候选源竞速中胜出；后续抓取因资产已存在
而跳过下载，无法自行恢复。

## 目标与验收

- 下载入口根据实际响应内容识别图片，拒绝 HTML、空响应和其他非图片内容；保留有效图片完整字节。
- 新增 migration 检查所有该 CDN 来源的未删除图片资产，保留有效文件，从其他候选源修复损坏或缺失
  文件，并更新资产的真实来源、对象路径、存储位置和状态。
- 单条失败不终止剩余扫描；失败必须阻止迁移完成记账，重试不遗漏未完成资产。
- 定向测试、独立评审及 lint 后发版、构建、推送并部署；生产备份、迁移统计及示例存档验证形成闭环。

## 范围

仅修改 tombkeeper 图片获取与本次历史数据迁移；不改微博正文解析、转发关系或其他来源的下载器。
