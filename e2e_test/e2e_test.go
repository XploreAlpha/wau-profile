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
	"testing"
	"time"

	profilev1 "github.com/wau/profile/profilev1"
	"github.com/wau/profile/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// startTestServer 启测试用 gRPC server(随机端口),返 (addr, cleanup)
func startTestServer(t *testing.T) (string, func()) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := lis.Addr().String()

	gs := grpc.NewServer()
	store := service.NewMemStore()
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
