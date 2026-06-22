// Package service wau-profile-service 核心实现
//
// v0.8.0 M2-1: 3 RPC(Get/Set/Delete Profile)+ D9 敏感字段拒收 + MemStore
// 跟 wau-intent 仓 M2-3 的 ProfileClient interface 完全对齐
package service

import (
	"fmt"
	"strings"
)

// Profile 用户画像(跟 wau-intent 仓 service/profile_client.go StaticProfile 同步)
//
// 字段含义(per v3 plan D8-D11):
//   - UserID: 主键(必填)
//   - Role: 角色(doctor / developer / trader / general,D11 冷启动用 "general")
//   - Department: 部门(healthcare / engineering / finance,D11 冷启动用 "unknown")
//   - PreferredSkills: 偏好技能(影响 wau-intent profile-aware 推荐)
//   - PreferredAgents: 偏好 agent(优先推荐)
//
// v0.8.0 不存敏感字段(D9 拒收 14 字段)
type Profile struct {
	UserID          string
	Role            string
	Department      string
	PreferredSkills []string
	PreferredAgents []string
}

// EmptyProfile 返回 D11 冷启动兜底 profile
//
// per v3 plan D11:冷启动场景(nil profile)用 EmptyProfile 兜底,
// 避免推荐逻辑因 nil profile panic。
//
// 注意:GetProfile 找不到时**不**自动返 EmptyProfile,让 caller 决定
// (Caller 拉不到就走通用推荐,不强制塞 empty profile)。
func EmptyProfile() *Profile {
	return &Profile{
		Role:       "general",
		Department: "unknown",
	}
}

// Clone 深拷贝(防止 caller 改返回值污染 cache)
//
// 设计:跟 wau-intent 仓 service/profile_client.go StaticProfile.Clone 对齐
func (p *Profile) Clone() *Profile {
	if p == nil {
		return nil
	}
	clone := &Profile{
		UserID:     p.UserID,
		Role:       p.Role,
		Department: p.Department,
	}
	if p.PreferredSkills != nil {
		clone.PreferredSkills = make([]string, len(p.PreferredSkills))
		copy(clone.PreferredSkills, p.PreferredSkills)
	}
	if p.PreferredAgents != nil {
		clone.PreferredAgents = make([]string, len(p.PreferredAgents))
		copy(clone.PreferredAgents, p.PreferredAgents)
	}
	return clone
}

// Validate 校验 Profile 基本合法性
//
// - UserID 必填
// - 不校验 D9(D9 校验在 sensitive.go 的 IsSensitiveFieldKey 单独做,
//   handler 入口调用并 wrap 错误码)
func (p *Profile) Validate() error {
	if p == nil {
		return fmt.Errorf("profile is nil")
	}
	if strings.TrimSpace(p.UserID) == "" {
		return fmt.Errorf("user_id required")
	}
	return nil
}
