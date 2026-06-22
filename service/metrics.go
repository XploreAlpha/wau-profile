// Package service Prometheus 指标
//
// v0.8.0 M2-2: 暴露 wau_profile_set_total / wau_profile_get_total / wau_profile_delete_total
//
// 跟 wau-scheduler / wau-intent 仓 prometheus 风格对齐
package service

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// profileSetTotal SetProfile 调用次数
	profileSetTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "wau_profile_set_total",
			Help: "Total SetProfile calls",
		},
		[]string{"tenant"},
	)

	// profileGetTotal GetProfile 调用次数(hit/miss/error)
	profileGetTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "wau_profile_get_total",
			Help: "Total GetProfile calls (hit/miss/error)",
		},
		[]string{"tenant", "result"},
	)

	// profileDeleteTotal DeleteProfile 调用次数
	profileDeleteTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "wau_profile_delete_total",
			Help: "Total DeleteProfile calls",
		},
		[]string{"tenant"},
	)

	// profileD9RejectTotal D9 拒收次数(在 handler 调,加在 metrics 包做集中注册)
	profileD9RejectTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "wau_profile_d9_reject_total",
			Help: "Total D9 sensitive field rejections",
		},
	)

	// profileTenantRejectTotal tenant 拒绝次数
	profileTenantRejectTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "wau_profile_tenant_reject_total",
			Help: "Total tenant rejections",
		},
		[]string{"tenant", "operation"},
	)
)

// init 注册所有指标
func init() {
	prometheus.MustRegister(
		profileSetTotal,
		profileGetTotal,
		profileDeleteTotal,
		profileD9RejectTotal,
		profileTenantRejectTotal,
	)
}
