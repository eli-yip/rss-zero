---
title: "图片下载需要验证实际内容，跨存储修复需要可重试"
plan: docs/plans/2026-09-03-tombkeeper-image-validation.md
status: done
updated: "2026-09-03"
---

HTTP 200 和非空内容不能证明下载的是图片。本次第三方代理返回 HTML 欢迎页，旧逻辑将它保存成
JPEG 并记为成功。采用 Go 标准库 `http.DetectContentType` 识别前 512 字节，在候选源竞争完成之前
拒绝非图片；流需回放已读取前缀并正确关闭原流。该校验负责类型识别，不负责完整像素解码。

迁移检查 MinIO 对象时，`GetObject` 返回流不代表对象可读，缺失、权限与网络错误可能延迟到第一次
Read。读取需要携带截止时间，并区分非图片、明确不存在和临时读取故障；只有前两类进入重下。

OSS 上传和数据库更新不是事务。先覆盖旧文件再更新来源，在数据库失败时可能导致下一次扫描看到
有效图片而跳过来源修复。因此新内容写入固定的修复路径，更新成功才切换数据库引用，旧对象保留。

独立 Spec 评审指出 SQL NULL 与 Go 零值的差异：历史 object_key/status/updated_at 可空，直接使用
等号匹配零值会导致条件更新永远失败。已通过真实 PostgreSQL 回归复现，再用 COALESCE 和
IS NOT DISTINCT FROM 修复，同时保留并发冲突检测。

本次定向测试包括下载拒绝回归、类型纠正、前缀和关闭语义、迁移分页与失败继续，以及隔离 PostgreSQL
中的资产更新、软删除过滤、并发冲突和 NULL 记录。lint 的既有 Go 1.27 改写建议仍在 xiaobot 原子计数器
和 tombkeeper mid/bid 反向遍历，本次不混入这些无关改动。

Standards 评审补充：不能将所有 `io.ErrUnexpectedEOF` 都视为正常短文件。使用
`io.ReadAll(io.LimitReader(body, 512))` 读取前缀，正常短流返回成功，底层异常截断仍保留为读取错误，
避免把临时断流当作非图片覆盖健康资产；对应回归已先复现失败再修复。
