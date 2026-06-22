package service

import "strings"

// =============================================================================
// v0.8.0 M2-1 D9 敏感字段拒收
// =============================================================================
//
// 14 字段 deny list(per v3 plan D9):
//   id_card, 身份证, ssn, passport, 护照,
//   credit_card, 卡号, cvv,
//   password, 密码, token, secret,
//   api_key, apikey
//
// 匹配规则:
//   1. 大小写不敏感(strings.ToLower)
//   2. 包含匹配(strings.Contains)防 user_credit_card 漏网
//
// wau-profile 是 source of truth,wau-intent 仓后续删本地 sensitive.go,
// 改 import github.com/wau/profile/service.IsSensitiveFieldKey
//
// =============================================================================

// denyFieldNames D9 14 字段(全小写)
var denyFieldNames = []string{
	"id_card",
	"身份证",
	"ssn",
	"passport",
	"护照",
	"credit_card",
	"卡号",
	"cvv",
	"password",
	"密码",
	"token",
	"secret",
	"api_key",
	"apikey",
}

// IsSensitiveFieldKey 判断字段 key 是否在 D9 14 字段 deny list 内
//
// 规则:
//   - 大小写不敏感
//   - 包含匹配(防 user_credit_card 漏网)
//
// 返回:true = 命中(应拒收)
func IsSensitiveFieldKey(key string) bool {
	if key == "" {
		return false
	}
	lower := strings.ToLower(key)
	for _, deny := range denyFieldNames {
		if strings.Contains(lower, deny) {
			return true
		}
	}
	return false
}

// CheckProfileSensitiveFields 校验 Profile 内所有字段是否含 D9 deny 字段
//
// 校验范围:Role / Department / PreferredSkills / PreferredAgents
// (注意:不校验 user_id,因为 user_id 必填,跟敏感字段语义冲突)
//
// 返回:第一个命中的字段名 + true(给 handler wrap error)
//      没命中返 "", false
func CheckProfileSensitiveFields(p *Profile) (string, bool) {
	if p == nil {
		return "", false
	}
	if IsSensitiveFieldKey(p.Role) {
		return "role", true
	}
	if IsSensitiveFieldKey(p.Department) {
		return "department", true
	}
	for _, skill := range p.PreferredSkills {
		if IsSensitiveFieldKey(skill) {
			return "preferred_skills[" + skill + "]", true
		}
	}
	for _, agent := range p.PreferredAgents {
		if IsSensitiveFieldKey(agent) {
			return "preferred_agents[" + agent + "]", true
		}
	}
	return "", false
}
