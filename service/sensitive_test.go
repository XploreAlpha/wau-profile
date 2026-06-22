package service

import "testing"

// TestIsSensitiveFieldKey:14 字段 + 大小写 + 包含匹配
func TestIsSensitiveFieldKey(t *testing.T) {
	denyCases := []string{
		"id_card", "身份证", "ssn", "passport", "护照",
		"credit_card", "卡号", "cvv",
		"password", "密码", "token", "secret",
		"api_key", "apikey",
		// 大小写变体
		"ID_CARD", "PassPort", "Credit_Card", "API_Key",
		// 包含匹配(防漏)
		"user_credit_card", "my_password_v2", "auth_token_2024",
		"api_key_secret", "primary_ssn",
	}
	for _, k := range denyCases {
		if !IsSensitiveFieldKey(k) {
			t.Errorf("IsSensitiveFieldKey(%q) 应返 true,实际 false", k)
		}
	}
}

// TestIsSensitiveFieldKey_Negative:合法字段不命中
func TestIsSensitiveFieldKey_Negative(t *testing.T) {
	allowCases := []string{
		"role", "department", "preferred_skills", "preferred_agents",
		"name", "email", "language", "formality",
		"doctor", "patient", "developer", "trader",
		"healthcare", "finance",
		"",
	}
	for _, k := range allowCases {
		if IsSensitiveFieldKey(k) {
			t.Errorf("IsSensitiveFieldKey(%q) 应返 false,实际 true", k)
		}
	}
}

// TestCheckProfileSensitiveFields:校验 4 字段位置(role/department/skills/agents)
func TestCheckProfileSensitiveFields(t *testing.T) {
	tests := []struct {
		name      string
		profile   *Profile
		wantField string
		wantHit   bool
	}{
		{
			"nil profile",
			nil,
			"", false,
		},
		{
			"role hit",
			&Profile{UserID: "u1", Role: "id_card"},
			"role", true,
		},
		{
			"department hit",
			&Profile{UserID: "u1", Department: "credit_card"},
			"department", true,
		},
		{
			"skill hit",
			&Profile{UserID: "u1", PreferredSkills: []string{"diagnosis", "my_password"}},
			"preferred_skills[my_password]", true,
		},
		{
			"agent hit",
			&Profile{UserID: "u1", PreferredAgents: []string{"Benny", "auth_token"}},
			"preferred_agents[auth_token]", true,
		},
		{
			"clean profile",
			&Profile{
				UserID:          "u1",
				Role:            "doctor",
				Department:      "healthcare",
				PreferredSkills: []string{"diagnosis"},
				PreferredAgents: []string{"Benny"},
			},
			"", false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, hit := CheckProfileSensitiveFields(tt.profile)
			if hit != tt.wantHit {
				t.Errorf("hit = %v, want %v", hit, tt.wantHit)
			}
			if field != tt.wantField {
				t.Errorf("field = %q, want %q", field, tt.wantField)
			}
		})
	}
}
