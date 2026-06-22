package service

import (
	"context"
	"testing"

	profilev1 "github.com/wau/profile/profilev1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newTestServer 构造测试用 handler
func newTestServer() *ProfileServiceServer {
	return NewProfileServiceServer(NewMemStore())
}

// =============================================================================
// GetProfile
// =============================================================================

// TestHandler_GetProfile_Success:正常拉画像
func TestHandler_GetProfile_Success(t *testing.T) {
	s := newTestServer()
	ctx := context.Background()

	// 先写入
	_, _ = s.SetProfile(ctx, &profilev1.SetProfileRequest{
		Profile: &profilev1.Profile{
			UserId:     "u1",
			Role:       "doctor",
			Department: "healthcare",
		},
	})

	// 拉取
	resp, err := s.GetProfile(ctx, &profilev1.GetProfileRequest{UserId: "u1"})
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if resp.GetNotFound() {
		t.Error("not_found should be false")
	}
	if resp.GetProfile().GetRole() != "doctor" {
		t.Errorf("role: got %q, want doctor", resp.GetProfile().GetRole())
	}
	if resp.GetSource() != "memory-store" {
		t.Errorf("source: got %q, want memory-store", resp.GetSource())
	}
}

// TestHandler_GetProfile_NotFound:找不到 → not_found=true, source=fallback-empty
func TestHandler_GetProfile_NotFound(t *testing.T) {
	s := newTestServer()
	resp, err := s.GetProfile(context.Background(), &profilev1.GetProfileRequest{UserId: "nonexistent"})
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if !resp.GetNotFound() {
		t.Error("not_found should be true")
	}
	if resp.GetSource() != "fallback-empty" {
		t.Errorf("source: got %q, want fallback-empty", resp.GetSource())
	}
	if resp.GetProfile() != nil {
		t.Error("profile should be nil when not found")
	}
}

// TestHandler_GetProfile_EmptyUserID:user_id 空 → InvalidArgument
func TestHandler_GetProfile_EmptyUserID(t *testing.T) {
	s := newTestServer()
	_, err := s.GetProfile(context.Background(), &profilev1.GetProfileRequest{UserId: ""})
	if err == nil {
		t.Fatal("应返 error")
	}
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code: got %v, want %v", code, codes.InvalidArgument)
	}
}

// =============================================================================
// SetProfile
// =============================================================================

// TestHandler_SetProfile_Success:正常写入
func TestHandler_SetProfile_Success(t *testing.T) {
	s := newTestServer()
	resp, err := s.SetProfile(context.Background(), &profilev1.SetProfileRequest{
		Profile: &profilev1.Profile{
			UserId:     "u1",
			Role:       "doctor",
			Department: "healthcare",
		},
	})
	if err != nil {
		t.Fatalf("SetProfile: %v", err)
	}
	if !resp.GetSuccess() {
		t.Errorf("success: got false, err_field=%q", resp.GetErrorField())
	}
}

// TestHandler_SetProfile_D9Rejected:role 含 deny 字段 → success=false, error_field="role"
func TestHandler_SetProfile_D9Rejected(t *testing.T) {
	s := newTestServer()
	resp, err := s.SetProfile(context.Background(), &profilev1.SetProfileRequest{
		Profile: &profilev1.Profile{
			UserId: "u1",
			Role:   "id_card",
		},
	})
	if err != nil {
		t.Fatalf("SetProfile 应不返 grpc error: %v", err)
	}
	if resp.GetSuccess() {
		t.Error("D9 拒收应返 success=false")
	}
	if resp.GetErrorField() != "role" {
		t.Errorf("error_field: got %q, want role", resp.GetErrorField())
	}
}

// TestHandler_SetProfile_D9Rejected_AllPositions:4 字段位置都测
func TestHandler_SetProfile_D9Rejected_AllPositions(t *testing.T) {
	tests := []struct {
		name      string
		profile   *profilev1.Profile
		wantField string
	}{
		{"role hit", &profilev1.Profile{UserId: "u1", Role: "id_card"}, "role"},
		{"department hit", &profilev1.Profile{UserId: "u1", Department: "credit_card"}, "department"},
		{"skill hit", &profilev1.Profile{UserId: "u1", PreferredSkills: []string{"my_password"}}, "preferred_skills[my_password]"},
		{"agent hit", &profilev1.Profile{UserId: "u1", PreferredAgents: []string{"auth_token"}}, "preferred_agents[auth_token]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer()
			resp, err := s.SetProfile(context.Background(), &profilev1.SetProfileRequest{Profile: tt.profile})
			if err != nil {
				t.Fatalf("SetProfile: %v", err)
			}
			if resp.GetSuccess() {
				t.Error("success should be false")
			}
			if resp.GetErrorField() != tt.wantField {
				t.Errorf("error_field: got %q, want %q", resp.GetErrorField(), tt.wantField)
			}
		})
	}
}

// TestHandler_SetProfile_EmptyUserID:user_id 空 → InvalidArgument
func TestHandler_SetProfile_EmptyUserID(t *testing.T) {
	s := newTestServer()
	_, err := s.SetProfile(context.Background(), &profilev1.SetProfileRequest{
		Profile: &profilev1.Profile{UserId: ""},
	})
	if err == nil {
		t.Fatal("应返 error")
	}
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code: got %v, want %v", code, codes.InvalidArgument)
	}
}

// TestHandler_SetProfile_NilProfile:profile nil → InvalidArgument
func TestHandler_SetProfile_NilProfile(t *testing.T) {
	s := newTestServer()
	_, err := s.SetProfile(context.Background(), &profilev1.SetProfileRequest{})
	if err == nil {
		t.Fatal("应返 error")
	}
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code: got %v, want %v", code, codes.InvalidArgument)
	}
}

// =============================================================================
// DeleteProfile
// =============================================================================

// TestHandler_DeleteProfile_Success:正常删除
func TestHandler_DeleteProfile_Success(t *testing.T) {
	s := newTestServer()
	ctx := context.Background()

	// 先写入
	_, _ = s.SetProfile(ctx, &profilev1.SetProfileRequest{
		Profile: &profilev1.Profile{UserId: "u1", Role: "doctor"},
	})

	// 删除
	resp, err := s.DeleteProfile(ctx, &profilev1.DeleteProfileRequest{UserId: "u1"})
	if err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	if !resp.GetSuccess() {
		t.Error("success should be true")
	}
	if resp.GetNotFound() {
		t.Error("not_found should be false")
	}

	// 再查应 not found
	getResp, _ := s.GetProfile(ctx, &profilev1.GetProfileRequest{UserId: "u1"})
	if !getResp.GetNotFound() {
		t.Error("GetProfile after Delete should be not_found")
	}
}

// TestHandler_DeleteProfile_NotFound:删除不存在的 user
func TestHandler_DeleteProfile_NotFound(t *testing.T) {
	s := newTestServer()
	resp, err := s.DeleteProfile(context.Background(), &profilev1.DeleteProfileRequest{UserId: "nonexistent"})
	if err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	if resp.GetSuccess() {
		t.Error("success should be false")
	}
	if !resp.GetNotFound() {
		t.Error("not_found should be true")
	}
}

// TestHandler_DeleteProfile_EmptyUserID:user_id 空 → InvalidArgument
func TestHandler_DeleteProfile_EmptyUserID(t *testing.T) {
	s := newTestServer()
	_, err := s.DeleteProfile(context.Background(), &profilev1.DeleteProfileRequest{UserId: ""})
	if err == nil {
		t.Fatal("应返 error")
	}
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code: got %v, want %v", code, codes.InvalidArgument)
	}
}

// =============================================================================
// 转换函数
// =============================================================================

// TestProfileToProto:深拷贝 slice
func TestProfileToProto(t *testing.T) {
	p := &Profile{
		UserID:          "u1",
		Role:            "doctor",
		PreferredSkills: []string{"diagnosis"},
	}
	proto := profileToProto(p)
	if proto.GetUserId() != "u1" {
		t.Errorf("user_id: got %q", proto.GetUserId())
	}
	if len(proto.GetPreferredSkills()) != 1 {
		t.Errorf("preferred_skills len: got %d", len(proto.GetPreferredSkills()))
	}
}

// TestProtoToProfile:nil 安全
func TestProtoToProfile(t *testing.T) {
	got := protoToProfile(nil)
	if got != nil {
		t.Errorf("nil proto 应返 nil profile,实际 %+v", got)
	}
}
