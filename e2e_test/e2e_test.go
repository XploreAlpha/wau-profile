// Package e2e_test wau-profile 端到端测试
//
// 流程:
//   1. 启 gRPC server(随机端口)
//   2. 用 grpc client 连
//   3. Set → Get → Delete → Get(not_found)流程
//   4. 测 D9 拒收
//   5. 测 tenant 隔离
//
// 跑法:
//   go test ./e2e_test/... -count=1
package e2e_test

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	profilev1 "github.com/wau/profile/profilev1"
	"github.com/wau/profile/service"

	"github.com/redis/go-redis/v9"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// newTestRedisClient e2e 用 Redis client(跟 service/store_redis_test 共享)
func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("WAU_PROFILE_REDIS_ADDR")
	if addr == "" {
		addr = "43.134.126.126:6379"
	}
	password := os.Getenv("WAU_TEST_REDIS_PASSWORD")
	client := redis.NewClient(&redis.Options{
		Addr:        addr,
		Password:    password,
		DB:          15, // 测试单独 db 15
		DialTimeout: 2 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("Redis not available at %s db=15: %v", addr, err)
	}
	_ = client.FlushDB(ctx).Err()
	t.Cleanup(func() {
		_ = client.FlushDB(context.Background()).Err()
		_ = client.Close()
	})
	return client
}

// startTestServer 启测试用 gRPC server(随机端口),返 (addr, cleanup)
func startTestServer(t *testing.T) (string, func()) {
	return startTestServerWithStore(t, service.NewMemStore(), nil)
}

// startTestServerWithStore 启测试用 gRPC server,带指定 store + tenant 白名单
//
// M2-2 加的 helper,允许 e2e test 测不同 store / 租户场景
//   - store: ProfileStore 实例
//   - allowedTenants: nil → 不过滤(只对 RedisStore 有意义)
func startTestServerWithStore(t *testing.T, store service.ProfileStore, allowedTenants []string) (string, func()) {
	t.Helper()

	// 如果 store 是 RedisStore,动态更新白名单
	if rs, ok := store.(*service.RedisStore); ok && allowedTenants != nil {
		rs.UpdateAllowedTenants(allowedTenants)
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := lis.Addr().String()

	gs := grpc.NewServer()
	ps := service.NewProfileServiceServer(store)
	profilev1.RegisterProfileServiceServer(gs, ps)
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(gs, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	reflection.Register(gs)

	go func() {
		_ = gs.Serve(lis)
	}()

	// 等 server ready
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cleanup := func() {
		gs.GracefulStop()
	}
	return addr, cleanup
}

// dialClient 建 client conn
func dialClient(t *testing.T, addr string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

// TestE2E_SetGetDelete:Set → Get → Delete → Get(not_found) 完整流程
func TestE2E_SetGetDelete(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	conn := dialClient(t, addr)
	defer conn.Close()
	client := profilev1.NewProfileServiceClient(conn)
	ctx := context.Background()

	// 1. Set
	setResp, err := client.SetProfile(ctx, &profilev1.SetProfileRequest{
		Profile: &profilev1.Profile{
			UserId:     "u1",
			Role:       "doctor",
			Department: "healthcare",
		},
	})
	if err != nil {
		t.Fatalf("SetProfile: %v", err)
	}
	if !setResp.GetSuccess() {
		t.Errorf("SetProfile success=false err_field=%q", setResp.GetErrorField())
	}

	// 2. Get
	getResp, err := client.GetProfile(ctx, &profilev1.GetProfileRequest{UserId: "u1"})
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if getResp.GetNotFound() {
		t.Error("GetProfile 刚写入应 not found=false")
	}
	if getResp.GetProfile().GetRole() != "doctor" {
		t.Errorf("role: got %q, want doctor", getResp.GetProfile().GetRole())
	}
	if getResp.GetSource() != "memory-store" {
		t.Errorf("source: got %q, want memory-store", getResp.GetSource())
	}

	// 3. Delete
	delResp, err := client.DeleteProfile(ctx, &profilev1.DeleteProfileRequest{UserId: "u1"})
	if err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	if !delResp.GetSuccess() {
		t.Error("DeleteProfile success=false")
	}

	// 4. Get 第二次应 not found
	getResp2, err := client.GetProfile(ctx, &profilev1.GetProfileRequest{UserId: "u1"})
	if err != nil {
		t.Fatalf("GetProfile 2nd: %v", err)
	}
	if !getResp2.GetNotFound() {
		t.Error("GetProfile after Delete 应 not found=true")
	}
}

// TestE2E_D9Reject:role 含 deny 字段 → SetProfile success=false
func TestE2E_D9Reject(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	conn := dialClient(t, addr)
	defer conn.Close()
	client := profilev1.NewProfileServiceClient(conn)
	ctx := context.Background()

	resp, err := client.SetProfile(ctx, &profilev1.SetProfileRequest{
		Profile: &profilev1.Profile{
			UserId: "u1",
			Role:   "credit_card",
		},
	})
	if err != nil {
		t.Fatalf("SetProfile 应不返 grpc error: %v", err)
	}
	if resp.GetSuccess() {
		t.Error("D9 拒收应 success=false")
	}
	if resp.GetErrorField() != "role" {
		t.Errorf("error_field: got %q, want role", resp.GetErrorField())
	}
}

// TestE2E_TenantIsolation:不同 tenant 同 user_id 应独立
func TestE2E_TenantIsolation(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	conn := dialClient(t, addr)
	defer conn.Close()
	client := profilev1.NewProfileServiceClient(conn)
	ctx := context.Background()

	// tenant-A 写 doctor
	_, _ = client.SetProfile(ctx, &profilev1.SetProfileRequest{
		TenantId: "tenant-A",
		Profile:  &profilev1.Profile{UserId: "u1", Role: "doctor"},
	})

	// tenant-B 写 developer
	_, _ = client.SetProfile(ctx, &profilev1.SetProfileRequest{
		TenantId: "tenant-B",
		Profile:  &profilev1.Profile{UserId: "u1", Role: "developer"},
	})

	// tenant-A 拉
	respA, _ := client.GetProfile(ctx, &profilev1.GetProfileRequest{
		TenantId: "tenant-A",
		UserId:   "u1",
	})
	if respA.GetProfile().GetRole() != "doctor" {
		t.Errorf("tenant-A role: got %q, want doctor", respA.GetProfile().GetRole())
	}

	// tenant-B 拉
	respB, _ := client.GetProfile(ctx, &profilev1.GetProfileRequest{
		TenantId: "tenant-B",
		UserId:   "u1",
	})
	if respB.GetProfile().GetRole() != "developer" {
		t.Errorf("tenant-B role: got %q, want developer", respB.GetProfile().GetRole())
	}
}

// TestE2E_HealthCheck:grpc.health.v1/Check 应返 SERVING
func TestE2E_HealthCheck(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	conn := dialClient(t, addr)
	defer conn.Close()
	healthClient := healthpb.NewHealthClient(conn)
	ctx := context.Background()

	resp, err := healthClient.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Health.Check: %v", err)
	}
	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Errorf("status: got %v, want SERVING", resp.GetStatus())
	}
}

// TestE2E_EmptyUserID:user_id 空 → InvalidArgument
func TestE2E_EmptyUserID(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	conn := dialClient(t, addr)
	defer conn.Close()
	client := profilev1.NewProfileServiceClient(conn)
	ctx := context.Background()

	// Get 空 user_id
	_, err := client.GetProfile(ctx, &profilev1.GetProfileRequest{UserId: ""})
	if err == nil {
		t.Error("GetProfile(user_id='') 应返 error")
	}

	// Set 空 user_id
	_, err = client.SetProfile(ctx, &profilev1.SetProfileRequest{
		Profile: &profilev1.Profile{UserId: ""},
	})
	if err == nil {
		t.Error("SetProfile(user_id='') 应返 error")
	}

	// Delete 空 user_id
	_, err = client.DeleteProfile(ctx, &profilev1.DeleteProfileRequest{UserId: ""})
	if err == nil {
		t.Error("DeleteProfile(user_id='') 应返 error")
	}
}

// ============== v0.8.0 M2-2 新增测试 ==============

// TestE2E_MetricsAreRegistered:prometheus 指标在 service 包内注册成功
//
// (不再从 e2e 测 HTTP /metrics — 那是 main.go 的职责,启动时验证)
// 单元测试 TestMetricsRegistered 在 metrics_test.go 覆盖
func TestE2E_MetricsAreRegistered(t *testing.T) {
	// 仅做 RPC 一次确保指标被触发;具体值检查留给单元测试
	addr, cleanup := startTestServer(t)
	defer cleanup()

	conn := dialClient(t, addr)
	defer conn.Close()
	client := profilev1.NewProfileServiceClient(conn)
	ctx := context.Background()

	_, _ = client.SetProfile(ctx, &profilev1.SetProfileRequest{
		Profile: &profilev1.Profile{UserId: "u1", Role: "doctor"},
	})
	_, _ = client.GetProfile(ctx, &profilev1.GetProfileRequest{UserId: "u1"})
}

// TestE2E_TenantReject_Get:不在白名单的 tenant Get → PermissionDenied
//
// 需要 RedisStore(MemStore 不做 tenant 校验),SkipIfNotAvailable
func TestE2E_TenantReject_Get(t *testing.T) {
	client := newTestRedisClient(t)
	rs := service.NewRedisStore(client, "wau:profile:v1:e2e:", []string{"default"}, 0)
	addr, cleanup := startTestServerWithStore(t, rs, []string{"default"})
	defer cleanup()

	conn := dialClient(t, addr)
	defer conn.Close()
	c := profilev1.NewProfileServiceClient(conn)
	ctx := context.Background()

	// tenant-A 不在白名单,Get 应 PermissionDenied
	_, err := c.GetProfile(ctx, &profilev1.GetProfileRequest{
		TenantId: "tenant-A",
		UserId:   "u1",
	})
	if err == nil {
		t.Fatal("forbidden tenant Get 应返 error")
	}
	if code := status.Code(err); code != codes.PermissionDenied {
		t.Errorf("code: got %v, want %v", code, codes.PermissionDenied)
	}
}

// TestE2E_TenantReject_Set:不在白名单的 tenant Set → PermissionDenied
func TestE2E_TenantReject_Set(t *testing.T) {
	client := newTestRedisClient(t)
	rs := service.NewRedisStore(client, "wau:profile:v1:e2e:", []string{"default"}, 0)
	addr, cleanup := startTestServerWithStore(t, rs, []string{"default"})
	defer cleanup()

	conn := dialClient(t, addr)
	defer conn.Close()
	c := profilev1.NewProfileServiceClient(conn)
	ctx := context.Background()

	_, err := c.SetProfile(ctx, &profilev1.SetProfileRequest{
		TenantId: "tenant-A",
		Profile:  &profilev1.Profile{UserId: "u1", Role: "doctor"},
	})
	if err == nil {
		t.Fatal("forbidden tenant Set 应返 error")
	}
	if code := status.Code(err); code != codes.PermissionDenied {
		t.Errorf("code: got %v, want %v", code, codes.PermissionDenied)
	}
}

// TestE2E_TenantAllowed_Default:空 tenant → 走 "default"
func TestE2E_TenantAllowed_Default(t *testing.T) {
	addr, cleanup := startTestServer(t)
	defer cleanup()

	conn := dialClient(t, addr)
	defer conn.Close()
	client := profilev1.NewProfileServiceClient(conn)
	ctx := context.Background()

	// 空 tenant → "default" 走通(MemStore 不做白名单校验,任何 tenant 都通)
	setResp, err := client.SetProfile(ctx, &profilev1.SetProfileRequest{
		Profile: &profilev1.Profile{UserId: "u1", Role: "doctor"},
	})
	if err != nil {
		t.Fatalf("SetProfile: %v", err)
	}
	if !setResp.GetSuccess() {
		t.Errorf("空 tenant SetProfile 应成功,实际 success=false err_field=%q", setResp.GetErrorField())
	}
}
