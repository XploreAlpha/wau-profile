package service

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// captureLogs 启动时替换默认 logger,捕获 slog 输出
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	old := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(old)
	})
	return buf
}

// TestAuditSet:AuditSet 应输出 JSON 含 event/tenant_id/user_id
func TestAuditSet(t *testing.T) {
	buf := captureLogs(t)
	AuditSet("tenant-A", "u1")

	out := buf.String()
	if !strings.Contains(out, "profile_set") {
		t.Errorf("log 应含 'profile_set',实际 %q", out)
	}

	// 验证是 JSON
	var log map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &log); err != nil {
		t.Errorf("log 应是 JSON,实际 err=%v output=%q", err, out)
	}
	if log["event"] != "profile_set" {
		t.Errorf("event: got %v, want profile_set", log["event"])
	}
	if log["tenant_id"] != "tenant-A" {
		t.Errorf("tenant_id: got %v, want tenant-A", log["tenant_id"])
	}
	if log["user_id"] != "u1" {
		t.Errorf("user_id: got %v, want u1", log["user_id"])
	}
}

// TestAuditDelete:AuditDelete 应输出 JSON 含 event/tenant_id/user_id
func TestAuditDelete(t *testing.T) {
	buf := captureLogs(t)
	AuditDelete("tenant-B", "u2")

	out := buf.String()
	if !strings.Contains(out, "profile_delete") {
		t.Errorf("log 应含 'profile_delete',实际 %q", out)
	}
}

// TestAuditRejectD9:AuditRejectD9 应输出 WARN 级 + field
func TestAuditRejectD9(t *testing.T) {
	buf := captureLogs(t)
	AuditRejectD9("tenant-A", "u1", "role")

	out := buf.String()
	if !strings.Contains(out, "profile_reject_d9") {
		t.Errorf("log 应含 'profile_reject_d9',实际 %q", out)
	}
	if !strings.Contains(out, "role") {
		t.Errorf("log 应含 'role',实际 %q", out)
	}
}

// TestAuditTenantReject:AuditTenantReject 应输出 WARN + operation
func TestAuditTenantReject(t *testing.T) {
	buf := captureLogs(t)
	AuditTenantReject("forbidden", "u1", "get")

	out := buf.String()
	if !strings.Contains(out, "profile_tenant_reject") {
		t.Errorf("log 应含 'profile_tenant_reject',实际 %q", out)
	}
	if !strings.Contains(out, "forbidden") {
		t.Errorf("log 应含 'forbidden',实际 %q", out)
	}
}
