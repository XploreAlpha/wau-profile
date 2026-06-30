# wau-profile 15 分钟跑通

> 目标:本机启 wau-profile + 写入 1 个用户 profile + 查询验证。

## 前置

- Go 1.21+
- 端口 :18480(gRPC ProfileClient)/ :18481(HTTP)
- OS:Linux / macOS / WSL2

## 步骤

### 1. 拉源码

```bash
cd ~/project/wau-profile
git pull origin main
make build
```

### 2. 配置

```bash
mkdir -p ~/.wau
cp configs/profile.yaml ~/.wau/

# 默认 SQLite(in-memory),prod 换 Postgres
# PROFILE_STORE_BACKEND=postgres PROFILE_STORE_DSN=$PROFILE_STORE_DSN
```

### 3. 启

```bash
./bin/wau-profile -config ~/.wau/profile.yaml
# 预期:[wau-profile] gRPC server starting on :18480
```

### 4. 写入 1 个 profile

```bash
grpcurl -plaintext -d '{
  "tenant_id": "acme",
  "user_id": "u-001",
  "preferences": {"language": "zh"},
  "tags": ["vip", "tech"]
}' \
  127.0.0.1:18480 wau.profile.v1.Profile/Upsert
```

### 5. 查询

```bash
grpcurl -plaintext -d '{
  "tenant_id": "acme",
  "user_id": "u-001"
}' \
  127.0.0.1:18480 wau.profile.v1.Profile/Get
```

预期:回显刚才写入的 preferences + tags

## 下一步

- [DEPLOY.md](DEPLOY.md) — Postgres 存储 + 备份策略
- [ARCHITECTURE.md](ARCHITECTURE.md) — Profile schema + 多租户
- [README.md](README.md) — v0.9.0 收口段
