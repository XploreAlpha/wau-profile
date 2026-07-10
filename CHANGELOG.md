## [v0.9.0] - 2026-07-02 (v0.9.0 GA)

### Highlights

- v0.9.0 同步发版 + 与 wau-channel/wau-edge/wau-llm-router 整合
- 详见 GA 收口报告:~/WAU-develop/develop-log/kernel/v0.9.0/wrapup/2026-07-02-PROGRESS-v0.9.0-GA-CLOSURE.md

### Compatibility

- API 100% 保留
- 4 SDK 同步 v1.2.0

## [Unreleased] — v1.0.0 "Phoenix" M11 P4.5 ⭐L5 包管理器 3 表 (2026-07-10, per D73)

### Added

- **SQL migration** `migrations/0001_l5_installed_agents.sql`:
  - `installed_agents` (user × agent × version) — JSONB manifest + Docker sandbox ID
  - `user_skill_pool`  (user × skill) — per-user enabled + source_agent
  - `agent_memory`     (user × agent × key) — KV 存储,JSONB value(卸不丢)
  - 索引(2-3 per 表)+ UNIQUE 约束
- **Go data model** `service/l5_store.go` (~390 LoC):
  - 3 struct: `InstalledAgent` / `UserSkill` / `AgentMemory`
  - `L5Store` interface (10 method) + `MemL5Store` in-memory dev/test 实现
  - 软删除(uninstall --purge=false 保留 SnapshotPath)
  - `GetInstalledAgent` 支持 `version='latest'` 返最新
  - `SetAgentMemory` KV 写保留 ID + CreatedAt

### Compatibility (D60 additive)

- `ProfileStore` interface 0 改
- `MemStore` / `RedisStore` 0 改
- 仅 v1.0.0 增量: 新加 `L5Store` interface + `MemL5Store` 实现 + 3 表 schema
- v0.9.0 老 5 SDK 不感知

### Verified

- `go test -race -count=1 ./...` **全 PASS, 0 回归**
- 21 个 L5 测试 PASS(MemL5Store installed_agents × 7 + user_skill_pool × 4 + agent_memory × 6 + Interface × 1)

### Reference

- D73 A 拍板 (wau-profile 升级范围 = 只新加 3 表, 0 改旧表):[stage1/01-D66-D74-9-decisions-summary#八](https://github.com/wau-network/WAU-develop/blob/main/develop-log/kernel/v1.0.0/stage1/01-D66-D74-9-decisions-summary.md)
- 详设:[stage1/05-wau-profile-3-tables-schema-design.md](https://github.com/wau-network/WAU-develop/blob/main/develop-log/kernel/v1.0.0/stage1/05-wau-profile-3-tables-schema-design.md)

# Changelog

wau-profile 倒序版本变更记录。

## [Unreleased] — v0.9.0 Stage 2 (2026-07-04)

### Added

- §3.8 docs/DEPLOY 骨架:`README.md` 末段 `v0.9.0 收口段` + 4 份新文件
- B 端 schema 规划(per [[project-wau-two-sided-marketplace-2026-06-27]],留 v1.0.0 实装)

### Compatibility

- ProfileClient interface 100% 兼容 v0.8.0

---

## [v0.8.0-sprout] — 2026-07-13

### Added

- v0.8.0 GA 发版
- SQLite 存储 + 缓存 + 多租户
- ProfileClient interface(wau-intent M2-3 aeb5dce)
