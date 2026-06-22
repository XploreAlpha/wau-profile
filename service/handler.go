package service

import (
	"context"
	"log/slog"

	profilev1 "github.com/wau/profile/profilev1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// =============================================================================
// v0.8.0 M2-1 ProfileService gRPC handler
// =============================================================================
//
// 3 RPC:
//   - GetProfile: 拉画像
//   - SetProfile: 写入画像(D9 拒收)
//   - DeleteProfile: 删除画像
//
// 设计:
//   - GetProfile 找不到 → GetProfileResponse{not_found: true},不返 EmptyProfile(让 caller 决定)
//   - D9 拒收 → codes.PermissionDenied + SetProfileResponse{success: false, error_field: "xxx"}
//   - Profile.user_id 空 → codes.InvalidArgument
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
		Source:  "memory-store",
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
		slog.Warn("SetProfile rejected by D9", "user_id", profile.UserID, "field", field)
		return &profilev1.SetProfileResponse{
			Success:    false,
			ErrorField: field,
		}, nil
	}

	if err := s.store.Set(ctx, req.GetTenantId(), profile); err != nil {
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
		slog.Error("DeleteProfile store error", "err", err, "user_id", req.GetUserId())
		return nil, status.Error(codes.Internal, "store error")
	}

	return &profilev1.DeleteProfileResponse{
		Success:  deleted,
		NotFound: !deleted,
	}, nil
}

// =============================================================================
// Profile <-> proto 转换
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
