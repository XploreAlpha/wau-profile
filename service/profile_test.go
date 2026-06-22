package service

import (
	"testing"
)

// TestEmptyProfile_ColdStart:EmptyProfile() 返 role=general / department=unknown
//
// 目的:per v3 plan D11,冷启动场景(nil profile)用 EmptyProfile 兜底,
// 避免推荐逻辑因 nil profile panic。
func TestEmptyProfile_ColdStart(t *testing.T) {
	p := EmptyProfile()
	if p == nil {
		t.Fatal("EmptyProfile() 应返非 nil")
	}
	if p.Role != "general" {
		t.Errorf("EmptyProfile.Role: got %q, want %q", p.Role, "general")
	}
	if p.Department != "unknown" {
		t.Errorf("EmptyProfile.Department: got %q, want %q", p.Department, "unknown")
	}
}

// TestProfile_Clone_Nil:nil receiver Clone 应返 nil
func TestProfile_Clone_Nil(t *testing.T) {
	var p *Profile
	got := p.Clone()
	if got != nil {
		t.Errorf("nil.Clone() 应返 nil,实际 %+v", got)
	}
}

// TestProfile_Clone_DeepCopy:Clone 后修改原 profile 不影响 clone
func TestProfile_Clone_DeepCopy(t *testing.T) {
	orig := &Profile{
		UserID:          "u1",
		Role:            "doctor",
		Department:      "healthcare",
		PreferredSkills: []string{"diagnosis"},
		PreferredAgents: []string{"Benny"},
	}
	clone := orig.Clone()
	orig.Role = "hacker"
	orig.PreferredSkills[0] = "malicious"

	if clone.Role != "doctor" {
		t.Errorf("Clone 不深拷贝 Role: got %q", clone.Role)
	}
	if clone.PreferredSkills[0] != "diagnosis" {
		t.Errorf("Clone 不深拷贝 PreferredSkills: got %v", clone.PreferredSkills)
	}
}

// TestProfile_Validate:user_id 必填
func TestProfile_Validate(t *testing.T) {
	tests := []struct {
		name    string
		profile *Profile
		wantErr bool
	}{
		{"nil profile", nil, true},
		{"empty user_id", &Profile{UserID: ""}, true},
		{"whitespace user_id", &Profile{UserID: "   "}, true},
		{"valid", &Profile{UserID: "u1", Role: "doctor"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.profile.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
