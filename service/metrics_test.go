package service

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestMetricsRegistered:验证 prometheus 指标都已注册且可增
//
// 用 prometheus testutil 抓取 counter 值,确认指标在 init() 注册成功
func TestMetricsRegistered(t *testing.T) {
	// 直接增 counter,看 testutil 能读到
	before := testutil.ToFloat64(profileSetTotal.WithLabelValues("test-tenant"))
	profileSetTotal.WithLabelValues("test-tenant").Inc()
	after := testutil.ToFloat64(profileSetTotal.WithLabelValues("test-tenant"))
	if after-before != 1 {
		t.Errorf("profileSetTotal 增 1: before=%v, after=%v", before, after)
	}

	// profileGetTotal 带 result label
	before2 := testutil.ToFloat64(profileGetTotal.WithLabelValues("test-tenant", "hit"))
	profileGetTotal.WithLabelValues("test-tenant", "hit").Inc()
	after2 := testutil.ToFloat64(profileGetTotal.WithLabelValues("test-tenant", "hit"))
	if after2-before2 != 1 {
		t.Errorf("profileGetTotal{hit} 增 1: before=%v, after=%v", before2, after2)
	}

	// profileDeleteTotal
	before3 := testutil.ToFloat64(profileDeleteTotal.WithLabelValues("test-tenant"))
	profileDeleteTotal.WithLabelValues("test-tenant").Inc()
	after3 := testutil.ToFloat64(profileDeleteTotal.WithLabelValues("test-tenant"))
	if after3-before3 != 1 {
		t.Errorf("profileDeleteTotal 增 1: before=%v, after=%v", before3, after3)
	}

	// profileD9RejectTotal
	before4 := testutil.ToFloat64(profileD9RejectTotal)
	profileD9RejectTotal.Inc()
	after4 := testutil.ToFloat64(profileD9RejectTotal)
	if after4-before4 != 1 {
		t.Errorf("profileD9RejectTotal 增 1: before=%v, after=%v", before4, after4)
	}

	// profileTenantRejectTotal
	before5 := testutil.ToFloat64(profileTenantRejectTotal.WithLabelValues("test-tenant", "get"))
	profileTenantRejectTotal.WithLabelValues("test-tenant", "get").Inc()
	after5 := testutil.ToFloat64(profileTenantRejectTotal.WithLabelValues("test-tenant", "get"))
	if after5-before5 != 1 {
		t.Errorf("profileTenantRejectTotal 增 1: before=%v, after=%v", before5, after5)
	}
}
