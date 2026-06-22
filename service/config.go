package service

import (
	"log/slog"
	"os"
)

// Config wau-profile-service 配置
// 全部从环境变量读,默认值适合本地开发
type Config struct {
	// gRPC 监听地址
	GRPCAddr string

	// 版本号(日志可观测)
	Version string
}

// LoadConfig 从 env 加载配置
func LoadConfig() *Config {
	return &Config{
		GRPCAddr: getEnv("WAU_PROFILE_GRPC_ADDR", ":50062"),
		Version:  getEnv("WAU_PROFILE_VERSION", "v0.8.0-M2-1"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// LogConfig slog 打印配置(启动时)
func (c *Config) LogConfig() {
	slog.Info("wau-profile-service config",
		"version", c.Version,
		"grpc_addr", c.GRPCAddr,
	)
}
