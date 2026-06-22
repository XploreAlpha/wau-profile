package service

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	profilev1 "github.com/wau/profile/profilev1"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// Run 启动 wau-profile-service
//
// 流程:
//   1. 加载配置
//   2. 设置 slog(JSON handler)
//   3. 初始化 store(M2-1 MemStore / M2-2 RedisStore,按 cfg.Backend 切换)
//   4. 构造 ProfileServiceServer
//   5. 启动 gRPC server(:50062)+ health + reflection
//   6. 启动 HTTP /healthz + /metrics(:8082,K8s 友好)
//   7. 等待 SIGINT/SIGTERM,graceful shutdown
func Run() error {
	// 1. 加载配置
	cfg := LoadConfig()

	// 2. 设置日志
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	cfg.LogConfig()

	// 3. 初始化 store(按 cfg.Backend 切换)
	store, err := newStore(cfg)
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}

	// 4. 构造 ProfileServiceServer
	ps := NewProfileServiceServer(store)

	// 5. 启动 gRPC server
	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.GRPCAddr, err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor),
	)
	profilev1.RegisterProfileServiceServer(grpcServer, ps)

	// 注册 health service(跟 wau-intent / wau-scheduler 一致)
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// 启用 reflection(grpcurl 调试用)
	reflection.Register(grpcServer)

	// 6. 启动 HTTP /healthz + /metrics 探针(K8s 友好)
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})
	httpMux.Handle("/metrics", promhttp.Handler())
	httpServer := &http.Server{Addr: ":8082", Handler: httpMux}

	// 7. 等待 SIGINT/SIGTERM
	errCh := make(chan error, 2)
	go func() {
		logger.Info("gRPC server listening", "addr", cfg.GRPCAddr)
		errCh <- grpcServer.Serve(lis)
	}()
	go func() {
		logger.Info("HTTP server listening", "addr", ":8082", "endpoints", "/healthz, /metrics")
		errCh <- httpServer.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("signal received, shutting down", "signal", sig)
	case err := <-errCh:
		if err != nil {
			logger.Error("server error", "err", err)
			return err
		}
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	grpcServer.GracefulStop()
	httpServer.Shutdown(shutdownCtx)
	logger.Info("wau-profile-service stopped")
	return nil
}

// newStore 按 cfg.Backend 构造 ProfileStore
//
// M2-1: "memory" → MemStore
// M2-2: "redis"  → RedisStore(连真 Redis,PING 失败返 error 启动失败)
func newStore(cfg *Config) (ProfileStore, error) {
	switch cfg.Backend {
	case "memory":
		return NewMemStore(), nil
	case "redis":
		if cfg.RedisAddr == "" {
			return nil, fmt.Errorf("WAU_PROFILE_BACKEND=redis but WAU_PROFILE_REDIS_ADDR empty")
		}
		client := redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword, // 从 env 读,可能为空(本地 redis 无密码)
			DB:       cfg.RedisDB,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := client.Ping(ctx).Err(); err != nil {
			return nil, fmt.Errorf("redis ping %s db=%d: %w", cfg.RedisAddr, cfg.RedisDB, err)
		}
		slog.Info("profile store initialized",
			"backend", "redis",
			"addr", cfg.RedisAddr,
			"db", cfg.RedisDB,
			"tenants", cfg.AllowedTenants,
		)
		return NewRedisStore(client, "wau:profile:v1:", cfg.AllowedTenants, cfg.TTL), nil
	default:
		return nil, fmt.Errorf("unknown WAU_PROFILE_BACKEND: %q (want memory|redis)", cfg.Backend)
	}
}

// loggingInterceptor 简单 gRPC 请求日志
func loggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	duration := time.Since(start)
	if err != nil {
		slog.Error("rpc failed", "method", info.FullMethod, "duration_ms", duration.Milliseconds(), "err", err)
	} else {
		slog.Info("rpc ok", "method", info.FullMethod, "duration_ms", duration.Milliseconds())
	}
	return resp, err
}
