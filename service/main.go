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
//   3. 初始化 store(M2-1 MemStore,M2-2 RedisStore)
//   4. 构造 ProfileServiceServer
//   5. 启动 gRPC server(:50062)+ health + reflection
//   6. 等待 SIGINT/SIGTERM,graceful shutdown
func Run() error {
	// 1. 加载配置
	cfg := LoadConfig()

	// 2. 设置日志
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	cfg.LogConfig()

	// 3. 初始化 store(M2-1 MemStore)
	store := NewMemStore()
	logger.Info("profile store initialized", "backend", "memory")

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

	// 启动 HTTP /healthz 探针(K8s 友好)
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})
	httpServer := &http.Server{Addr: ":8082", Handler: httpMux}

	// 6. 等待 SIGINT/SIGTERM
	errCh := make(chan error, 2)
	go func() {
		logger.Info("gRPC server listening", "addr", cfg.GRPCAddr)
		errCh <- grpcServer.Serve(lis)
	}()
	go func() {
		logger.Info("HTTP /healthz listening", "addr", ":8082")
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
