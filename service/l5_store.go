// Package service - l5_store.go
//
// v1.0.0 M11 P4.5 ⭐L5 包管理器 3 表 Go data model (per D73, 2026-07-10)。
//
// D73 = A 拍板:wau-profile 升级范围 = 只新加 3 表,0 改 v0.9.0 旧表
//   - installed_agents  (user × agent)
//   - user_skill_pool   (user × skill)
//   - agent_memory      (user × agent × key)
//
// 本文件提供 Go 端 data model + L5Store interface (in-memory Mem 实现)。
// SQL schema 见 migrations/0001_l5_installed_agents.sql (Postgres-ready)。
//
// D60 additive:0 改 ProfileStore interface;0 改 v0.9.0 MemStore / RedisStore。
//
// ahead of W10 末 deadline: ~3 个月
package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrL5NotFound L5 record not found
var ErrL5NotFound = errors.New("l5: record not found")

// ErrL5AlreadyExists L5 record already exists
var ErrL5AlreadyExists = errors.New("l5: record already exists")

// InstalledAgent ⭐L5 installed_agents 行
type InstalledAgent struct {
	ID              int64     `json:"id"`
	UserID          string    `json:"user_id"`
	AgentName       string    `json:"agent_name"`
	AgentVersion    string    `json:"agent_version"`
	ManifestYAML    string    `json:"manifest_yaml"`     // 完整 manifest YAML 备份
	SandboxDockerID string    `json:"sandbox_docker_id"` // 当前 Docker container ID (per D68)
	Enabled         bool      `json:"enabled"`
	InstalledAt     time.Time `json:"installed_at"`
	UninstalledAt   *time.Time `json:"uninstalled_at,omitempty"`
	SnapshotPath    string    `json:"snapshot_path,omitempty"` // 软删除时的 snapshot 路径
}

// UserSkill ⭐L5 user_skill_pool 行
type UserSkill struct {
	ID           int64     `json:"id"`
	UserID       string    `json:"user_id"`
	SkillName    string    `json:"skill_name"`
	SkillVersion string    `json:"skill_version"`
	SourceAgent  string    `json:"source_agent,omitempty"` // 哪个 agent 注入
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
}

// AgentMemory ⭐L5 agent_memory 行(KV 存储)
type AgentMemory struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	AgentName string    `json:"agent_name"`
	Key       string    `json:"mem_key"`
	Value     string    `json:"mem_value"` // JSON-encoded
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// L5Store ⭐L5 3 表数据访问接口
//
// D60 additive:跟 ProfileStore 并列(独立 interface)。
// 实现:
//   - MemL5Store: in-memory(本期 dev/test,本期最小集)
//   - RedisL5Store: v1.x 后续(v0.9.0 已经用 Redis,延续)
//   - PostgresL5Store: v1.x 后续(对应 migrations/0001 SQL)
type L5Store interface {
	// installed_agents CRUD
	RegisterInstalledAgent(ctx context.Context, a *InstalledAgent) error
	GetInstalledAgent(ctx context.Context, userID, agentName, version string) (*InstalledAgent, error)
	ListInstalledAgents(ctx context.Context, userID string, enabledOnly bool) ([]*InstalledAgent, error)
	UninstallInstalledAgent(ctx context.Context, userID, agentName string, purge bool) (*InstalledAgent, error) // 返 soft-deleted record

	// user_skill_pool CRUD
	AddUserSkill(ctx context.Context, s *UserSkill) error
	ListUserSkills(ctx context.Context, userID string, enabledOnly bool) ([]*UserSkill, error)
	RemoveUserSkill(ctx context.Context, userID, skillName string) error

	// agent_memory KV CRUD
	SetAgentMemory(ctx context.Context, m *AgentMemory) error
	GetAgentMemory(ctx context.Context, userID, agentName, key string) (*AgentMemory, error)
	ListAgentMemory(ctx context.Context, userID, agentName string) ([]*AgentMemory, error)
	DeleteAgentMemory(ctx context.Context, userID, agentName, key string) error
}

// =============================================================================
// MemL5Store: in-memory 实现(本期最小集,跟 MemStore 模式一致)
// =============================================================================

// MemL5Store in-memory L5Store,带 sync.RWMutex 并发安全
type MemL5Store struct {
	mu sync.RWMutex

	// installed_agents 索引:userID × (agentName × version) → *InstalledAgent
	installed map[string]map[string]*InstalledAgent

	// user_skill_pool 索引:userID × skillName → *UserSkill
	skills map[string]map[string]*UserSkill

	// agent_memory 索引:userID × agentName × key → *AgentMemory
	memory map[string]map[string]map[string]*AgentMemory

	// ID 计数器
	nextInstalledID int64
	nextSkillID     int64
	nextMemoryID    int64
}

// NewMemL5Store 构造空 store
func NewMemL5Store() *MemL5Store {
	return &MemL5Store{
		installed:       make(map[string]map[string]*InstalledAgent),
		skills:          make(map[string]map[string]*UserSkill),
		memory:          make(map[string]map[string]map[string]*AgentMemory),
		nextInstalledID: 1,
		nextSkillID:     1,
		nextMemoryID:    1,
	}
}

// RegisterInstalledAgent 装 agent(per InstallAgent RPC)
func (s *MemL5Store) RegisterInstalledAgent(ctx context.Context, a *InstalledAgent) error {
	if a.UserID == "" || a.AgentName == "" || a.AgentVersion == "" {
		return errors.New("l5: user_id, agent_name, agent_version are required")
	}
	if a.InstalledAt.IsZero() {
		a.InstalledAt = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	userMap, ok := s.installed[a.UserID]
	if !ok {
		userMap = make(map[string]*InstalledAgent)
		s.installed[a.UserID] = userMap
	}
	key := a.AgentName + "|" + a.AgentVersion
	if _, exists := userMap[key]; exists {
		return ErrL5AlreadyExists
	}
	s.nextInstalledID++
	a.ID = s.nextInstalledID
	userMap[key] = a
	return nil
}

// GetInstalledAgent 查 installed_agent
func (s *MemL5Store) GetInstalledAgent(ctx context.Context, userID, agentName, version string) (*InstalledAgent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	userMap, ok := s.installed[userID]
	if !ok {
		return nil, ErrL5NotFound
	}
	if version == "" || version == "latest" {
		// 返最新 enabled 的一个
		var latest *InstalledAgent
		for _, a := range userMap {
			if a.AgentName != agentName {
				continue
			}
			if a.UninstalledAt != nil {
				continue
			}
			if latest == nil || a.InstalledAt.After(latest.InstalledAt) {
				latest = a
			}
		}
		if latest == nil {
			return nil, ErrL5NotFound
		}
		clone := *latest
		return &clone, nil
	}
	a, exists := userMap[agentName+"|"+version]
	if !exists {
		return nil, ErrL5NotFound
	}
	clone := *a
	return &clone, nil
}

// ListInstalledAgents 列已装 agent
func (s *MemL5Store) ListInstalledAgents(ctx context.Context, userID string, enabledOnly bool) ([]*InstalledAgent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	userMap, ok := s.installed[userID]
	if !ok {
		return nil, nil
	}
	out := make([]*InstalledAgent, 0, len(userMap))
	for _, a := range userMap {
		if enabledOnly && a.UninstalledAt != nil {
			continue
		}
		clone := *a
		out = append(out, &clone)
	}
	return out, nil
}

// UninstallInstalledAgent 卸 agent(purge=true 真删,否则软删 + snapshot path)
func (s *MemL5Store) UninstallInstalledAgent(ctx context.Context, userID, agentName string, purge bool) (*InstalledAgent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	userMap, ok := s.installed[userID]
	if !ok {
		return nil, ErrL5NotFound
	}
	// 找最新 enabled 的一个
	var target *InstalledAgent
	for _, a := range userMap {
		if a.AgentName != agentName {
			continue
		}
		if a.UninstalledAt != nil {
			continue
		}
		if target == nil || a.InstalledAt.After(target.InstalledAt) {
			target = a
		}
	}
	if target == nil {
		return nil, ErrL5NotFound
	}
	now := time.Now()
	if purge {
		delete(userMap, target.AgentName+"|"+target.AgentVersion)
		return target, nil
	}
	// 软删 + snapshot
	target.UninstalledAt = &now
	target.SnapshotPath = "/var/lib/wau-agent/snapshots/" + userID + "_" + agentName + "_" + now.Format("20060102150405") + ".tar.gz"
	clone := *target
	return &clone, nil
}

// AddUserSkill 加 skill
func (s *MemL5Store) AddUserSkill(ctx context.Context, sk *UserSkill) error {
	if sk.UserID == "" || sk.SkillName == "" {
		return errors.New("l5: user_id and skill_name are required")
	}
	if sk.CreatedAt.IsZero() {
		sk.CreatedAt = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	userMap, ok := s.skills[sk.UserID]
	if !ok {
		userMap = make(map[string]*UserSkill)
		s.skills[sk.UserID] = userMap
	}
	s.nextSkillID++
	sk.ID = s.nextSkillID
	userMap[sk.SkillName] = sk
	return nil
}

// ListUserSkills 列 user skills
func (s *MemL5Store) ListUserSkills(ctx context.Context, userID string, enabledOnly bool) ([]*UserSkill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	userMap, ok := s.skills[userID]
	if !ok {
		return nil, nil
	}
	out := make([]*UserSkill, 0, len(userMap))
	for _, sk := range userMap {
		if enabledOnly && !sk.Enabled {
			continue
		}
		clone := *sk
		out = append(out, &clone)
	}
	return out, nil
}

// RemoveUserSkill 删 skill
func (s *MemL5Store) RemoveUserSkill(ctx context.Context, userID, skillName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	userMap, ok := s.skills[userID]
	if !ok {
		return ErrL5NotFound
	}
	if _, exists := userMap[skillName]; !exists {
		return ErrL5NotFound
	}
	delete(userMap, skillName)
	return nil
}

// SetAgentMemory 写 KV
func (s *MemL5Store) SetAgentMemory(ctx context.Context, m *AgentMemory) error {
	if m.UserID == "" || m.AgentName == "" || m.Key == "" {
		return errors.New("l5: user_id, agent_name, mem_key are required")
	}
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	s.mu.Lock()
	defer s.mu.Unlock()
	agentMap, ok := s.memory[m.UserID]
	if !ok {
		agentMap = make(map[string]map[string]*AgentMemory)
		s.memory[m.UserID] = agentMap
	}
	keyMap, ok := agentMap[m.AgentName]
	if !ok {
		keyMap = make(map[string]*AgentMemory)
		agentMap[m.AgentName] = keyMap
	}
	if existing, exists := keyMap[m.Key]; exists {
		// 保留 ID + CreatedAt,只更新 Value + UpdatedAt
		existing.Value = m.Value
		existing.UpdatedAt = now
		return nil
	}
	s.nextMemoryID++
	m.ID = s.nextMemoryID
	keyMap[m.Key] = m
	return nil
}

// GetAgentMemory 读 KV
func (s *MemL5Store) GetAgentMemory(ctx context.Context, userID, agentName, key string) (*AgentMemory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agentMap, ok := s.memory[userID]
	if !ok {
		return nil, ErrL5NotFound
	}
	keyMap, ok := agentMap[agentName]
	if !ok {
		return nil, ErrL5NotFound
	}
	m, exists := keyMap[key]
	if !exists {
		return nil, ErrL5NotFound
	}
	clone := *m
	return &clone, nil
}

// ListAgentMemory 列 agent memory
func (s *MemL5Store) ListAgentMemory(ctx context.Context, userID, agentName string) ([]*AgentMemory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	agentMap, ok := s.memory[userID]
	if !ok {
		return nil, nil
	}
	keyMap, ok := agentMap[agentName]
	if !ok {
		return nil, nil
	}
	out := make([]*AgentMemory, 0, len(keyMap))
	for _, m := range keyMap {
		clone := *m
		out = append(out, &clone)
	}
	return out, nil
}

// DeleteAgentMemory 删 KV
func (s *MemL5Store) DeleteAgentMemory(ctx context.Context, userID, agentName, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	agentMap, ok := s.memory[userID]
	if !ok {
		return ErrL5NotFound
	}
	keyMap, ok := agentMap[agentName]
	if !ok {
		return ErrL5NotFound
	}
	if _, exists := keyMap[key]; !exists {
		return ErrL5NotFound
	}
	delete(keyMap, key)
	return nil
}