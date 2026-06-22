package service

import (
	"context"
	"log/slog"
	"strings"

	profilev1 "github.com/wau/profile/profilev1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// =============================================================================
// v0.8.0 M2-2 ProfileService gRPC handler
// =============================================================================
//
// M2-2 改动:
//   - 跟 MemStore / RedisStore 兼容(handler 不知道具体 store 类型)
//   - SetProfile D9 拒收时调 AuditRejectD9 + profileD9RejectTotal
//   - store 返 tenant 拒绝 error → 返 codes.PermissionDenied
//   - source tag 按 store 类型(redis / memory-store)动态判断
//
// =============================================================================

// ProfileServiceServer 完整 gRPC server 实现
type ProfileServiceServer struct {
	profilev1.UnimplementedProfileServiceServer
	store ProfileStore
}

// NewProfileServiceServer 构造 handler
func NewProfileServiceServer(store ProfileStore) *ProfileServiceServer {
	return &ProfileServiceServer{store: store}
}

// =============================================================================
// GetProfile
// =============================================================================

func (s *ProfileServiceServer) GetProfile(ctx context.Context, req *profilev1.GetProfileRequest) (*profilev1.GetProfileResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}

	profile, found, err := s.store.Get(ctx, req.GetTenantId(), req.GetUserId())
	if err != nil {
		if isTenantReject(err) {
			AuditTenantReject(req.GetTenantId(), req.GetUserId(), "get")
			profileTenantRejectTotal.WithLabelValues(req.GetTenantId(), "get").Inc()
			return nil, status.Errorf(codes.PermissionDenied, "tenant not allowed: %v", err)
		}
		slog.Error("GetProfile store error", "err", err, "user_id", req.GetUserId())
		return nil, status.Error(codes.Internal, "store error")
	}

	if !found {
		return &profilev1.GetProfileResponse{
			NotFound: true,
			Source:   "fallback-empty",
		}, nil
	}

	return &profilev1.GetProfileResponse{
		Profile: profileToProto(profile),
		Source:  sourceOfStore(s.store),
	}, nil
}

// =============================================================================
// SetProfile
// =============================================================================

func (s *ProfileServiceServer) SetProfile(ctx context.Context, req *profilev1.SetProfileRequest) (*profilev1.SetProfileResponse, error) {
	if req.GetProfile() == nil {
		return nil, status.Error(codes.InvalidArgument, "profile required")
	}

	profile := protoToProfile(req.GetProfile())
	if err := profile.Validate(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid profile: %v", err)
	}

	// D9 敏感字段校验
	if field, hit := CheckProfileSensitiveFields(profile); hit {
		AuditRejectD9(req.GetTenantId(), profile.UserID, field)
		profileD9RejectTotal.Inc()
		return &profilev1.SetProfileResponse{
			Success:    false,
			ErrorField: field,
		}, nil
	}

	if err := s.store.Set(ctx, req.GetTenantId(), profile); err != nil {
		if isTenantReject(err) {
			AuditTenantReject(req.GetTenantId(), profile.UserID, "set")
			profileTenantRejectTotal.WithLabelValues(req.GetTenantId(), "set").Inc()
			return nil, status.Errorf(codes.PermissionDenied, "tenant not allowed: %v", err)
		}
		slog.Error("SetProfile store error", "err", err, "user_id", profile.UserID)
		return nil, status.Error(codes.Internal, "store error")
	}

	return &profilev1.SetProfileResponse{
		Success: true,
	}, nil
}

// =============================================================================
// DeleteProfile
// =============================================================================

func (s *ProfileServiceServer) DeleteProfile(ctx context.Context, req *profilev1.DeleteProfileRequest) (*profilev1.DeleteProfileResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id required")
	}

	deleted, err := s.store.Delete(ctx, req.GetTenantId(), req.GetUserId())
	if err != nil {
		if isTenantReject(err) {
			AuditTenantReject(req.GetTenantId(), req.GetUserId(), "delete")
			profileTenantRejectTotal.WithLabelValues(req.GetTenantId(), "delete").Inc()
			return nil, status.Errorf(codes.PermissionDenied, "tenant not allowed: %v", err)
		}
		slog.Error("DeleteProfile store error", "err", err, "user_id", req.GetUserId())
		return nil, status.Error(codes.Internal, "store error")
	}

	return &profilev1.DeleteProfileResponse{
		Success:  deleted,
		NotFound: !deleted,
	}, nil
}

// =============================================================================
// helpers
// =============================================================================

// isTenantReject 判断是否是 tenant 拒绝 error
//
// Store 返 "tenant %q not allowed" 错误,handler 转 grpc PermissionDenied
func isTenantReject(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "not allowed") || strings.Contains(msg, "tenant")
}

// sourceOfStore 返 store 类型的 source tag
func sourceOfStore(store ProfileStore) string {
	switch store.(type) {
	case *RedisStore:
		return "redis"
	case *MemStore:
		return "memory-store"
	default:
		return "unknown"
	}
}

// =============================================================================
// Profile <-> proto 转换(跟 M2-1 一致)
// =============================================================================

func profileToProto(p *Profile) *profilev1.Profile {
	if p == nil {
		return nil
	}
	return &profilev1.Profile{
		UserId:          p.UserID,
		Role:            p.Role,
		Department:      p.Department,
		PreferredSkills: p.PreferredSkills,
		PreferredAgents: p.PreferredAgents,
	}
}

func protoToProfile(p *profilev1.Profile) *Profile {
	if p == nil {
		return nil
	}
	return &Profile{
		UserID:          p.GetUserId(),
		Role:            p.GetRole(),
		Department:      p.GetDepartment(),
		PreferredSkills: p.GetPreferredSkills(),
		PreferredAgents: p.GetPreferredAgents(),
	}
}
