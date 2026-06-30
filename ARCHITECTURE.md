# wau-profile 架构

## 模块拆分

```
wau-profile/
├── cmd/wau-profile/main.go       # 主入口
├── internal/
│   ├── config/                    # YAML 配置
│   ├── store/                     # storage backend(SQLite / Postgres)
│   ├── cache/                     # profile 缓存
│   ├── multitenancy/              # tenant 隔离
│   └── metrics/                   # prom 指标占位
├── proto/                         # ProfileClient interface
├── configs/profile.yaml
├── tests/
└── README.md / QUICKSTART.md / DEPLOY.md / ARCHITECTURE.md / CHANGELOG.md
```

## 数据流

```
WAU-core-kernel / wau-intent / wau-channel (gRPC Get / Upsert)
    ↓
wau-profile 缓存(命中)→ 直接返回
                ↓ 未命中
               store backend(SQLite / Postgres)
```

## 关键决策

| 决策 | 内容 |
|---|---|
| **6 子模块之一** | per [[project-wau-core-product-list-2026-06-28]] |
| **ProfileClient interface** | wau-intent M2-3 commit aeb5dce 已对齐 |
| **B 端 vision** | per [[project-wau-two-sided-marketplace-2026-06-27]] |

## 接口边界

- **入**:gRPC ProfileClient.Get / Upsert / Delete / List
- **出**:profile struct + tenant_id + user_id 维度
- **依赖**:存储 backend(SQLite / Postgres)
- **被依赖**:WAU-core-kernel / wau-intent / wau-channel

## Profile schema(per tenant + user_id)

```yaml
profile:
  tenant_id: "acme"
  user_id: "u-001"
  preferences:
    language: "zh"
    theme: "dark"
  tags:
    - "vip"
    - "tech"
  history_summary: "(internal, B 端可见)"
  last_active: "2026-07-04T10:00:00Z"
```

## 性能预算

| 指标 | 目标 |
|---|---|
| Get P50 | < 1 ms(命中缓存)|
| Upsert P50 | < 10 ms |
| 缓存命中率 | > 80% |

## 跟其他仓的关系

- **上游(调用本仓)**:WAU-core-kernel / wau-intent / wau-channel / B 端 dashboard
- **下游**:存储 backend
- **同组**:wau-scheduler / wau-trust / wau-circuit / wau-intent / wau-registry / wau-registry-service
