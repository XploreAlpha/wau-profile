package service

import (
	"os"
	"testing"
)

// TestLoadConfig_Default:env 都空时用默认值
func TestLoadConfig_Default(t *testing.T) {
	// 清理 env
	os.Unsetenv("WAU_PROFILE_GRPC_ADDR")
	os.Unsetenv("WAU_PROFILE_VERSION")

	cfg := LoadConfig()
	if cfg.GRPCAddr != ":50062" {
		t.Errorf("GRPCAddr: got %q, want :50062", cfg.GRPCAddr)
	}
	if cfg.Version != "v0.8.0-M2-1" {
		t.Errorf("Version: got %q, want v0.8.0-M2-1", cfg.Version)
	}
}

// TestLoadConfig_Override:env 覆盖
func TestLoadConfig_Override(t *testing.T) {
	os.Setenv("WAU_PROFILE_GRPC_ADDR", ":9999")
	os.Setenv("WAU_PROFILE_VERSION", "test-version")
	defer os.Unsetenv("WAU_PROFILE_GRPC_ADDR")
	defer os.Unsetenv("WAU_PROFILE_VERSION")

	cfg := LoadConfig()
	if cfg.GRPCAddr != ":9999" {
		t.Errorf("GRPCAddr: got %q, want :9999", cfg.GRPCAddr)
	}
	if cfg.Version != "test-version" {
		t.Errorf("Version: got %q, want test-version", cfg.Version)
	}
}
