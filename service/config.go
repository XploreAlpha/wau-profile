package service

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config wau-profile-service 配置
// 全部从环境变量读,默认值适合本地开发
type Config struct {
	// gRPC 监听地址
	GRPCAddr string

	// 版本号(日志可观测)
	Version string

	// ====================================================================
	// M2-2 新增:Redis 主存配置
	// ====================================================================

	// Backend: "memory" / "redis"
	//   - memory: M2-1 默认(开发用)
	//   - redis:  M2-2 推荐(生产用,持久化 + 跨实例共享)
	Backend string

	// RedisAddr Redis 地址,默认 43.134.126.126:6379(per 2026-06-21 memory)
	RedisAddr string

	// RedisPassword Redis 密码(从 env 读,不 echo 完整值,per feedback-redis-password-leak-2026-06-21)
	RedisPassword string

	// RedisDB Redis db number,默认 2(避免跟 dispatcher / wau-registry / wau-scheduler 串)
	RedisDB int

	// AllowedTenants 允许的 tenant 白名单
	//   - 默认 ["default"](M2-1 兼容)
	//   - 逗号分隔 env:WAU_PROFILE_ALLOWED_TENANTS=default,tenant-A,tenant-B
	AllowedTenants []string

	// TTL 0 = 永不过期,>0 = Set 时带 TTL
	TTL time.Duration
}

// LoadConfig 从 env 加载配置
func LoadConfig() *Config {
	cfg := &Config{
		GRPCAddr:      getEnv("WAU_PROFILE_GRPC_ADDR", ":50062"),
		Version:       getEnv("WAU_PROFILE_VERSION", "v0.8.0-M2-2"),
		Backend:       getEnv("WAU_PROFILE_BACKEND", "memory"),
		RedisAddr:     getEnv("WAU_PROFILE_REDIS_ADDR", "43.134.126.126:6379"),
		RedisPassword: os.Getenv("WAU_PROFILE_REDIS_PASSWORD"), // 不设默认
	}

	// RedisDB
	if v := os.Getenv("WAU_PROFILE_REDIS_DB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RedisDB = n
		} else {
			cfg.RedisDB = 2
		}
	} else {
		cfg.RedisDB = 2
	}

	// AllowedTenants
	if v := os.Getenv("WAU_PROFILE_ALLOWED_TENANTS"); v != "" {
		parts := strings.Split(v, ",")
		var tenants []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				tenants = append(tenants, p)
			}
		}
		cfg.AllowedTenants = tenants
	}
	if len(cfg.AllowedTenants) == 0 {
		cfg.AllowedTenants = []string{"default"}
	}

	// TTL
	if v := os.Getenv("WAU_PROFILE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.TTL = d
		}
	}
	// 默认 0(永不过期)

	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// LogConfig slog 打印配置(启动时)
// 注意:不打 RedisPassword 完整值(per feedback-redis-password-leak-2026-06-21)
func (c *Config) LogConfig() {
	slog.Info("wau-profile-service config",
		"version", c.Version,
		"grpc_addr", c.GRPCAddr,
		"backend", c.Backend,
		"redis_addr", c.RedisAddr,
		"redis_db", c.RedisDB,
		"redis_password_set", c.RedisPassword != "",
		"allowed_tenants", c.AllowedTenants,
		"ttl", c.TTL,
	)
}
