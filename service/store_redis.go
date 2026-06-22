// Package service Redis 主存实现
//
// v0.8.0 M2-2: MemStore → RedisStore(Redis 主存 + tenant 强校验 + audit + metrics)
//
// Key 格式: prefix + tenant_id + ":" + user_id
//   e.g. "wau:profile:v1:tenant-A:u1"
//
// Tenant 校验:在 Store 层做白名单(不在白名单的 tenant 拒绝 CRUD)
//
// D9 敏感字段拒收:不在 Store 层做(职责单一,在 handler 层)
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore Redis 主存实现(实现 ProfileStore interface)
type RedisStore struct {
	client         *redis.Client
	prefix         string
	allowedTenants map[string]bool
	ttl            time.Duration
	mu             sync.RWMutex // 保护 allowedTenants(动态更新可能)
}

// NewRedisStore 创建 RedisStore
//
//   - prefix: key 前缀,默认 "wau:profile:v1:"
//   - allowedTenants: 允许的 tenant 白名单(空 → 只有 "default")
//   - ttl: 0 = 永不过期,>0 = Set 时带 TTL
func NewRedisStore(client *redis.Client, prefix string, allowedTenants []string, ttl time.Duration) *RedisStore {
	if prefix == "" {
		prefix = "wau:profile:v1:"
	}
	allowed := make(map[string]bool)
	if len(allowedTenants) == 0 {
		allowed["default"] = true
	} else {
		for _, t := range allowedTenants {
			if t != "" {
				allowed[t] = true
			}
		}
		// "default" 始终允许(向后兼容 M2-1)
		if !allowed["default"] {
			allowed["default"] = true
		}
	}
	return &RedisStore{
		client:         client,
		prefix:         prefix,
		allowedTenants: allowed,
		ttl:            ttl,
	}
}

// key 构造完整 key
func (s *RedisStore) key(tenantID, userID string) string {
	return s.prefix + tenantID + ":" + userID
}

// checkTenant 校验 tenant 是否在白名单
//
// 返回 error if 不在白名单
// 空 tenant 自动走 "default"(M2-1 兼容)
func (s *RedisStore) checkTenant(tenantID string) error {
	if tenantID == "" {
		tenantID = "default"
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.allowedTenants[tenantID] {
		return fmt.Errorf("tenant %q not allowed", tenantID)
	}
	return nil
}

// Get 拉画像(Redis 实现)
func (s *RedisStore) Get(ctx context.Context, tenantID, userID string) (*Profile, bool, error) {
	if userID == "" {
		return nil, false, fmt.Errorf("user_id required")
	}
	if err := s.checkTenant(tenantID); err != nil {
		return nil, false, err
	}

	val, err := s.client.Get(ctx, s.key(tenantID, userID)).Result()
	if errors.Is(err, redis.Nil) {
		profileGetTotal.WithLabelValues(tenantID, "miss").Inc()
		return nil, false, nil
	}
	if err != nil {
		profileGetTotal.WithLabelValues(tenantID, "error").Inc()
		return nil, false, fmt.Errorf("redis get: %w", err)
	}

	var p Profile
	if err := json.Unmarshal([]byte(val), &p); err != nil {
		profileGetTotal.WithLabelValues(tenantID, "error").Inc()
		return nil, false, fmt.Errorf("json unmarshal: %w", err)
	}
	profileGetTotal.WithLabelValues(tenantID, "hit").Inc()
	return p.Clone(), true, nil
}

// Set 写入画像(Redis 实现)
func (s *RedisStore) Set(ctx context.Context, tenantID string, p *Profile) error {
	if p == nil {
		return fmt.Errorf("profile is nil")
	}
	if err := p.Validate(); err != nil {
		return err
	}
	if err := s.checkTenant(tenantID); err != nil {
		return err
	}

	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	if err := s.client.Set(ctx, s.key(tenantID, p.UserID), data, s.ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	profileSetTotal.WithLabelValues(tenantID).Inc()
	AuditSet(tenantID, p.UserID)
	return nil
}

// Delete 删除画像(Redis 实现)
func (s *RedisStore) Delete(ctx context.Context, tenantID, userID string) (bool, error) {
	if userID == "" {
		return false, fmt.Errorf("user_id required")
	}
	if err := s.checkTenant(tenantID); err != nil {
		return false, err
	}

	n, err := s.client.Del(ctx, s.key(tenantID, userID)).Result()
	if err != nil {
		return false, fmt.Errorf("redis del: %w", err)
	}
	if n == 0 {
		return false, nil
	}
	profileDeleteTotal.WithLabelValues(tenantID).Inc()
	AuditDelete(tenantID, userID)
	return true, nil
}

// Health 健康检查(Redis 实现:PING)
func (s *RedisStore) Health(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// UpdateAllowedTenants 动态更新白名单(管理端用,测试用)
func (s *RedisStore) UpdateAllowedTenants(tenants []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	allowed := make(map[string]bool)
	for _, t := range tenants {
		if t != "" {
			allowed[t] = true
		}
	}
	if !allowed["default"] {
		allowed["default"] = true
	}
	s.allowedTenants = allowed
}
