package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// =============================================================================
// v0.8.0 M2-1 ProfileStore interface + MemStore 实现
// =============================================================================
//
// ProfileStore 是后端抽象:
//   - M2-1: NewMemStore()(in-memory,带 sync.RWMutex)
//   - M2-2: NewRedisStore(...)(Redis 主存 + tenant 强校验 + audit log)
//
// Tenant 隔离:key 格式 "profile:{tenant_id}:{user_id}"
//   - M2-1 接受 tenant_id 参数但不强制校验
//   - M2-2 强制白名单(caller tenant 必须在 allowed 列表)
// =============================================================================

// ProfileStore 后端存储接口
type ProfileStore interface {
	// Get 拉画像
	//   - 找到:返 (clone, true, nil)
	//   - 找不到:返 (nil, false, nil)
	//   - 错误:返 (nil, false, err)
	Get(ctx context.Context, tenantID, userID string) (*Profile, bool, error)

	// Set 写入画像
	//   - 成功:返 nil
	//   - 错误:返 err
	Set(ctx context.Context, tenantID string, p *Profile) error

	// Delete 删除画像
	//   - 找到并删除:返 (true, nil)
	//   - 没找到:返 (false, nil)
	//   - 错误:返 (false, err)
	Delete(ctx context.Context, tenantID, userID string) (bool, error)

	// Health 健康检查
	Health(ctx context.Context) error
}

// ErrNotFound profile 不存在(可选,handler 也可只用 bool)
var ErrNotFound = errors.New("profile not found")

// makeKey 构造 tenant-scoped key
func makeKey(tenantID, userID string) string {
	if tenantID == "" {
		tenantID = "default"
	}
	return fmt.Sprintf("profile:%s:%s", tenantID, userID)
}

// =============================================================================
// MemStore 内存实现(M2-1 用)
// =============================================================================

// MemStore in-memory store,带 sync.RWMutex 保证并发安全
//
// 重要:Get 返 deep clone,防 caller 改返回值污染 cache
type MemStore struct {
	mu      sync.RWMutex
	entries map[string]*Profile // key = makeKey(tenantID, userID)
}

// NewMemStore 创建 MemStore
func NewMemStore() *MemStore {
	return &MemStore{
		entries: make(map[string]*Profile),
	}
}

// Get 拉画像(M2-1 内存实现)
func (m *MemStore) Get(ctx context.Context, tenantID, userID string) (*Profile, bool, error) {
	if userID == "" {
		return nil, false, fmt.Errorf("user_id required")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.entries[makeKey(tenantID, userID)]
	if !ok {
		return nil, false, nil
	}
	return p.Clone(), true, nil
}

// Set 写入画像(M2-1 内存实现)
func (m *MemStore) Set(ctx context.Context, tenantID string, p *Profile) error {
	if p == nil {
		return fmt.Errorf("profile is nil")
	}
	if err := p.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// 写入时也存 clone,防 caller 后续改入参影响 cache
	m.entries[makeKey(tenantID, p.UserID)] = p.Clone()
	return nil
}

// Delete 删除画像(M2-1 内存实现)
func (m *MemStore) Delete(ctx context.Context, tenantID, userID string) (bool, error) {
	if userID == "" {
		return false, fmt.Errorf("user_id required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := makeKey(tenantID, userID)
	if _, ok := m.entries[key]; !ok {
		return false, nil
	}
	delete(m.entries, key)
	return true, nil
}

// Health 健康检查(M2-1 内存实现永远 OK)
func (m *MemStore) Health(ctx context.Context) error {
	return nil
}

// Size 当前 entry 数(测试用)
func (m *MemStore) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}
