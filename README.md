# wau-profile

> WAU 网络的用户画像 Profile 服务 - 支持 tenant 隔离 + 敏感字段拒收 + Redis 主存

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)

---

## 核心职责

提供**用户画像 (User Profile)** 的 gRPC CRUD 服务,是 v0.8.0 M2 (Profile 模块) 的核心后端。

- **Get / Set / Delete** 三个 RPC
- **D9 敏感字段拒收**:14 字段 deny list (id_card / 密码 / token / api_key 等)
- **Tenant 隔离**:不同 tenant 的 user_id 互不干扰 (`profile:{tenant_id}:{user_id}`)
- **存储后端**:M2-1 in-memory / M2-2 Redis 主存

```
┌─────────────┐  GetProfile(user_id, tenant_id)
│ wau-intent  │ ─────────────────────────────► ┌──────────────┐
│ (M2-3)      │ ◄─────── Profile ────────────  │  wau-profile │
└─────────────┘                                │  (本仓)       │
                                                │  gRPC :50062 │
                                                └──────────────┘
```

---

## proto 契约

3 个 RPC,定义在 [`proto/wau_profile.proto`](proto/wau_profile.proto):

```protobuf
service ProfileService {
  rpc GetProfile(GetProfileRequest) returns (GetProfileResponse);
  rpc SetProfile(SetProfileRequest) returns (SetProfileResponse);
  rpc DeleteProfile(DeleteProfileRequest) returns (DeleteProfileResponse);
}
```

`Profile` 字段:
- `user_id` (主键,必填)
- `role` (doctor / developer / trader / general)
- `department` (healthcare / engineering / finance / unknown)
- `preferred_skills` / `preferred_agents` (推荐系统偏好)

> ⚠️ v0.8.0 **不存敏感字段**(id_card / 密码 / token 等)。SetProfile 时 D9 校验,命中 → `codes.PermissionDenied` + `error_field` 指示具体字段。

---

## D9 敏感字段 deny list (14 字段)

`id_card` / `身份证` / `ssn` / `passport` / `护照` / `credit_card` / `卡号` / `cvv` / `password` / `密码` / `token` / `secret` / `api_key` / `apikey`

`IsSensitiveFieldKey(key)` 规则:
- 大小写不敏感
- **包含匹配**(防 `user_credit_card` 漏网)
- 命中 → Profile 写入被拒,`error_field` wrap 具体字段名

---

## 跑起来

### 本地开发 (M2-1 in-memory)

```bash
# 1. 生成 proto
buf generate

# 2. 跑 service
go run ./cmd/wau-profile-service
# 默认监听 :50062,WAU_PROFILE_GRPC_ADDR 可覆盖

# 3. 健康检查
grpcurl -plaintext localhost:50062 grpc.health.v1.Health/Check
# {"status":"SERVING"}
```

### 集成到 wau-intent (M2-2 之后)

```bash
# wau-intent 仓 service/main.go
WAU_INTENT_PROFILE_ENABLED=true \
WAU_INTENT_PROFILE_GRPC_ADDR=localhost:50062 \
go run ./cmd/wau-intent-service
```

---

## 配置 (env)

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `WAU_PROFILE_GRPC_ADDR` | `:50062` | gRPC 监听地址 |
| `WAU_PROFILE_VERSION` | `v0.8.0-M2-1` | 版本号(日志) |

M2-2 新增:
- `WAU_PROFILE_REDIS_ADDR` (默认 `43.134.126.126:6379`)
- `WAU_PROFILE_REDIS_PASSWORD` (从 env 读,不 echo)

---

## 路线图

| 版本 | 内容 |
|------|------|
| **v0.8.0 M2-1** ✅ | 脚手架 + 3 RPC + MemStore + D9 拒收 + e2e |
| **v0.8.0 M2-2** | Redis 主存 + tenant 强校验 + audit log |
| **v0.8.0 M2-3** | wau-intent 切 GrpcProfileClient(替换 noopEnabledClient) |
| **v1.0.0** | TLS + 多 region 同步 + GDPR 完全合规 |

---

## 仓结构

```
wau-profile/
├── proto/wau_profile.proto         # 3 RPC + Profile + 5 message
├── profilev1/                      # buf generate 产物
├── service/
│   ├── profile.go                  # StaticProfile + EmptyProfile + Clone
│   ├── sensitive.go                # D9 14 字段 deny
│   ├── store.go                    # ProfileStore interface + MemStore
│   ├── handler.go                  # ProfileService gRPC 实现
│   ├── config.go                   # env 配置
│   ├── handler_test.go             # 3 RPC 单测
│   ├── profile_test.go             # 冷启动 + 深拷贝
│   ├── sensitive_test.go           # 14 字段
│   ├── store_test.go               # CRUD + 并发
│   ├── config_test.go              # env 覆盖
│   └── main.go                     # Run() 启 gRPC + health
├── cmd/wau-profile-service/main.go # entry point
├── e2e_test/e2e_test.go            # client → server 端到端
└── scripts/buf.sh                  # buf generate 一键脚本
```

---

## 参考

- v3 plan: `/home/inamoto888/WAU-develop/develop-log/kernel/v0.8.0/2026-06-21-v0.8.0-development-plan.md`
- M2-1 计划: `/home/inamoto888/WAU-develop/plan/kernel/v0.8.0/2026-06-22-M2-1-wau-profile-scaffold.md`
- wau-intent M2-3 commit: `aeb5dce` (ProfileClient interface + D9 + D10 + D11)
- 同构参考: `wau-scheduler` 仓(本仓结构同构)

---

## v0.9.0 "Acorn" 收口段(2026-09-15 GA)

上文详细介绍了 v0.8.0 雏形 + ProfileClient interface + 同构参考。本段为 v0.9.0 GA 增量补充。

### 角色

| OS 类比 | User Profile Store(用户画像存储)|
|---|---|
| 部署 | 独立 git 仓 = `wau-profile`,WAU-core-kernel 6 子模块之一 |
| 通信 | gRPC ProfileClient(被 wau-intent / WAU-core-kernel 调用)|
| 状态 | v0.8.0 GA 已发(2026-07-13)|

### v0.9.0 集成(per B 端 vision)

- **2-sided marketplace**(per [[project-wau-two-sided-marketplace-2026-06-27]]):
  - C 端 profile:用户偏好 + 历史 + 偏好模型
  - B 端 profile:商家画像 + 客户群体分析
- **B 端 dashboard 数据源**:registration_count, active_users, GMV 指标从本仓读
- **3 新仓接入**:wau-channel 把 C 端消息打 tag 存 profile;wau-intent 读 profile 做 routing

### v0.9.0 "Acorn" 5 份核心文档

| # | 文件 | 内容 |
|---|---|---|
| 1 | [README.md](README.md)(本文件)| 仓入口 + ProfileClient + v0.9.0 收口段 |
| 2 | [QUICKSTART.md](QUICKSTART.md) | 15 分钟跑通 1 个 Profile 写入 + 查询 |
| 3 | [DEPLOY.md](DEPLOY.md) | 存储 backend + 备份策略 |
| 4 | [ARCHITECTURE.md](ARCHITECTURE.md) | Profile schema + 多租户隔离 |
| 5 | [CHANGELOG.md](CHANGELOG.md) | v0.8.0 + v0.9.0 倒序 |

### 历史锚点

- v0.8.0 GA([[project-v0.8.0-GA-2026-07-13]])
- 2-sided vision([[project-wau-two-sided-marketplace-2026-06-27]])
