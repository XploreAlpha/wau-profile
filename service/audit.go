// Package service audit log
//
// v0.8.0 M2-2: 记录 Set/Delete 操作(谁改了什么,GDPR 友好)
//
// audit log 通过 slog.Info 输出(走 JSON handler,标准日志收集可解析)
// 不写入 Redis(避免额外存储成本,slog 输出到 stdout 即可)
package service

import (
	"log/slog"
	"time"
)

// AuditSet 记录 SetProfile 操作
func AuditSet(tenantID, userID string) {
	slog.Info("profile_set",
		"event", "profile_set",
		"tenant_id", tenantID,
		"user_id", userID,
		"ts", time.Now().UTC().Format(time.RFC3339),
	)
}

// AuditDelete 记录 DeleteProfile 操作
func AuditDelete(tenantID, userID string) {
	slog.Info("profile_delete",
		"event", "profile_delete",
		"tenant_id", tenantID,
		"user_id", userID,
		"ts", time.Now().UTC().Format(time.RFC3339),
	)
}

// AuditRejectD9 记录 D9 拒收(放在 handler 层调用)
func AuditRejectD9(tenantID, userID, field string) {
	slog.Warn("profile_reject_d9",
		"event", "profile_reject_d9",
		"tenant_id", tenantID,
		"user_id", userID,
		"field", field,
		"ts", time.Now().UTC().Format(time.RFC3339),
	)
}

// AuditTenantReject 记录 tenant 拒绝
func AuditTenantReject(tenantID, userID, operation string) {
	slog.Warn("profile_tenant_reject",
		"event", "profile_tenant_reject",
		"tenant_id", tenantID,
		"user_id", userID,
		"operation", operation,
		"ts", time.Now().UTC().Format(time.RFC3339),
	)
}
