package service

import (
	"testing"
)

// TestLoadConfig_Default:env 都空时用默认值
func TestLoadConfig_Default(t *testing.T) {
	// t.Setenv 自动清理
	t.Setenv("WAU_PROFILE_GRPC_ADDR", "")
	t.Setenv("WAU_PROFILE_VERSION", "")
	t.Setenv("WAU_PROFILE_BACKEND", "")
	t.Setenv("WAU_PROFILE_REDIS_ADDR", "")
	t.Setenv("WAU_PROFILE_REDIS_PASSWORD", "")
	t.Setenv("WAU_PROFILE_REDIS_DB", "")
	t.Setenv("WAU_PROFILE_ALLOWED_TENANTS", "")
	t.Setenv("WAU_PROFILE_TTL", "")

	cfg := LoadConfig()
	if cfg.GRPCAddr != ":50062" {
		t.Errorf("GRPCAddr: got %q, want :50062", cfg.GRPCAddr)
	}
	if cfg.Version != "v0.8.0-M2-2" {
		t.Errorf("Version: got %q, want v0.8.0-M2-2", cfg.Version)
	}
	if cfg.Backend != "memory" {
		t.Errorf("Backend: got %q, want memory", cfg.Backend)
	}
	if cfg.RedisAddr != "43.134.126.126:6379" {
		t.Errorf("RedisAddr: got %q, want 43.134.126.126:6379", cfg.RedisAddr)
	}
	if cfg.RedisDB != 2 {
		t.Errorf("RedisDB: got %d, want 2", cfg.RedisDB)
	}
	if len(cfg.AllowedTenants) != 1 || cfg.AllowedTenants[0] != "default" {
		t.Errorf("AllowedTenants: got %v, want [default]", cfg.AllowedTenants)
	}
	if cfg.TTL != 0 {
		t.Errorf("TTL: got %v, want 0", cfg.TTL)
	}
}

// TestLoadConfig_Override:env 覆盖
func TestLoadConfig_Override(t *testing.T) {
	t.Setenv("WAU_PROFILE_GRPC_ADDR", ":9999")
	t.Setenv("WAU_PROFILE_VERSION", "test-version")
	t.Setenv("WAU_PROFILE_BACKEND", "redis")
	t.Setenv("WAU_PROFILE_REDIS_ADDR", "localhost:9999")
	t.Setenv("WAU_PROFILE_REDIS_PASSWORD", "test-pass")
	t.Setenv("WAU_PROFILE_REDIS_DB", "5")
	t.Setenv("WAU_PROFILE_ALLOWED_TENANTS", "default,tenant-A,tenant-B")
	t.Setenv("WAU_PROFILE_TTL", "30m")

	cfg := LoadConfig()
	if cfg.GRPCAddr != ":9999" {
		t.Errorf("GRPCAddr: got %q, want :9999", cfg.GRPCAddr)
	}
	if cfg.Backend != "redis" {
		t.Errorf("Backend: got %q, want redis", cfg.Backend)
	}
	if cfg.RedisAddr != "localhost:9999" {
		t.Errorf("RedisAddr: got %q, want localhost:9999", cfg.RedisAddr)
	}
	if cfg.RedisPassword != "test-pass" {
		t.Errorf("RedisPassword: got %q, want test-pass", cfg.RedisPassword)
	}
	if cfg.RedisDB != 5 {
		t.Errorf("RedisDB: got %d, want 5", cfg.RedisDB)
	}
	if len(cfg.AllowedTenants) != 3 {
		t.Errorf("AllowedTenants: got %v, want 3 items", cfg.AllowedTenants)
	}
	if cfg.TTL.String() != "30m0s" {
		t.Errorf("TTL: got %v, want 30m", cfg.TTL)
	}
}

// TestLoadConfig_EmptyAllowedTenants:空 env 时默认 ["default"]
func TestLoadConfig_EmptyAllowedTenants(t *testing.T) {
	t.Setenv("WAU_PROFILE_ALLOWED_TENANTS", "")

	cfg := LoadConfig()
	if len(cfg.AllowedTenants) != 1 || cfg.AllowedTenants[0] != "default" {
		t.Errorf("空 ALLOWED_TENANTS 应默认 [default],实际 %v", cfg.AllowedTenants)
	}
}

// TestLoadConfig_InvalidDB:无效 DB 时 fallback 2
func TestLoadConfig_InvalidDB(t *testing.T) {
	t.Setenv("WAU_PROFILE_REDIS_DB", "not-a-number")

	cfg := LoadConfig()
	if cfg.RedisDB != 2 {
		t.Errorf("无效 DB 应 fallback 2,实际 %d", cfg.RedisDB)
	}
}
