# wau-profile 部署

## 端口

| 端口 | 类型 | 端点 |
|---|---|---|
| 18480 | gRPC | `wau.profile.v1.Profile` |
| 18481 | HTTP | `/healthz` + `/metrics` |

## 存储 backend

默认 SQLite(开发)。生产用 Postgres:

```bash
PROFILE_STORE_BACKEND=postgres
PROFILE_STORE_DSN=postgres://user:pass@host:5432/wau_profile?sslmode=require
```

**DSN 密码用 `$PROFILE_STORE_DSN` 占位**(per [[feedback-redis-password-leak-2026-06-21]])

## 监控

```bash
curl -s http://localhost:18481/metrics | grep wau_profile
```

## 进程管理

```bash
tmux new -d -s wau-profile '/tmp/wau-profile -config ~/.wau/profile.yaml'
```

## 配置

| 字段 | 默认 | 说明 |
|---|---|---|
| `store.backend` | `sqlite` | `sqlite` / `postgres` |
| `store.dsn` | `:memory:` | DB DSN |
| `cache.ttl_seconds` | `300` | profile 缓存 TTL |
| `multitenancy.isolation` | `tenant_id` | tenant_id / tenant_id+user_id |

## 升级路径

- v0.9.0(Acorn)→ v0.8.0(Sprout):
  - schema 100% 兼容
- v0.9.0 → v1.0.0(per [[project-wau-two-sided-marketplace-2026-06-27]]):
  - B 端 profile 二级 schema
  - 多 tier(基础 / 高级)
