// Package service - l5_store_test.go
//
// v1.0.0 M11 P4.5 ⭐L5 3 表 MemL5Store 测试 (per D73, 2026-07-10)。
//
// 覆盖:installed_agents / user_skill_pool / agent_memory 3 表全 CRUD。
package service

import (
	"context"
	"errors"
	"testing"
)

// ============================================================
// installed_agents 测试
// ============================================================

// TestL5_RegisterInstalledAgent_Success
func TestL5_RegisterInstalledAgent_Success(t *testing.T) {
	store := NewMemL5Store()
	a := &InstalledAgent{
		UserID: "alice", AgentName: "weather-agent", AgentVersion: "1.2.3",
		ManifestYAML: "name: weather-agent\nversion: 1.2.3\n",
		SandboxDockerID: "docker-abc",
		Enabled:         true,
	}
	if err := store.RegisterInstalledAgent(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if a.ID == 0 {
		t.Error("ID not set")
	}
	if a.InstalledAt.IsZero() {
		t.Error("InstalledAt not set")
	}
}

// TestL5_RegisterInstalledAgent_Required
func TestL5_RegisterInstalledAgent_Required(t *testing.T) {
	store := NewMemL5Store()
	cases := []InstalledAgent{
		{AgentName: "x", AgentVersion: "1"},
		{UserID: "alice", AgentVersion: "1"},
		{UserID: "alice", AgentName: "x"},
	}
	for _, a := range cases {
		if err := store.RegisterInstalledAgent(context.Background(), &a); err == nil {
			t.Errorf("expected error for %+v", a)
		}
	}
}

// TestL5_RegisterInstalledAgent_Duplicate
func TestL5_RegisterInstalledAgent_Duplicate(t *testing.T) {
	store := NewMemL5Store()
	a := &InstalledAgent{UserID: "alice", AgentName: "weather", AgentVersion: "1.0"}
	if err := store.RegisterInstalledAgent(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	a2 := &InstalledAgent{UserID: "alice", AgentName: "weather", AgentVersion: "1.0"}
	if err := store.RegisterInstalledAgent(context.Background(), a2); !errors.Is(err, ErrL5AlreadyExists) {
		t.Errorf("err = %v, want ErrL5AlreadyExists", err)
	}
}

// TestL5_GetInstalledAgent_Latest
func TestL5_GetInstalledAgent_Latest(t *testing.T) {
	store := NewMemL5Store()
	ctx := context.Background()
	for _, v := range []string{"1.0.0", "1.1.0", "1.2.0"} {
		if err := store.RegisterInstalledAgent(ctx, &InstalledAgent{
			UserID: "alice", AgentName: "weather", AgentVersion: v,
		}); err != nil {
			t.Fatal(err)
		}
	}
	a, err := store.GetInstalledAgent(ctx, "alice", "weather", "latest")
	if err != nil {
		t.Fatal(err)
	}
	if a.AgentVersion != "1.2.0" {
		t.Errorf("got %q, want 1.2.0", a.AgentVersion)
	}
}

// TestL5_GetInstalledAgent_SpecificVersion
func TestL5_GetInstalledAgent_SpecificVersion(t *testing.T) {
	store := NewMemL5Store()
	ctx := context.Background()
	if err := store.RegisterInstalledAgent(ctx, &InstalledAgent{
		UserID: "alice", AgentName: "weather", AgentVersion: "1.0.0",
	}); err != nil {
		t.Fatal(err)
	}
	a, err := store.GetInstalledAgent(ctx, "alice", "weather", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if a.AgentVersion != "1.0.0" {
		t.Errorf("got %q", a.AgentVersion)
	}
}

// TestL5_GetInstalledAgent_NotFound
func TestL5_GetInstalledAgent_NotFound(t *testing.T) {
	store := NewMemL5Store()
	_, err := store.GetInstalledAgent(context.Background(), "alice", "x", "1.0")
	if !errors.Is(err, ErrL5NotFound) {
		t.Errorf("err = %v, want ErrL5NotFound", err)
	}
}

// TestL5_ListInstalledAgents_EnabledOnly
func TestL5_ListInstalledAgents_EnabledOnly(t *testing.T) {
	store := NewMemL5Store()
	ctx := context.Background()
	for _, name := range []string{"weather", "reminder"} {
		if err := store.RegisterInstalledAgent(ctx, &InstalledAgent{
			UserID: "alice", AgentName: name, AgentVersion: "1.0",
		}); err != nil {
			t.Fatal(err)
		}
	}
	// 软删 weather
	if _, err := store.UninstallInstalledAgent(ctx, "alice", "weather", false); err != nil {
		t.Fatal(err)
	}
	all, _ := store.ListInstalledAgents(ctx, "alice", false)
	if len(all) != 2 {
		t.Errorf("all: got %d, want 2", len(all))
	}
	enabled, _ := store.ListInstalledAgents(ctx, "alice", true)
	if len(enabled) != 1 || enabled[0].AgentName != "reminder" {
		t.Errorf("enabled: got %d, want 1 (reminder)", len(enabled))
	}
}

// TestL5_UninstallInstalledAgent_Default_KeepsSnapshot
func TestL5_UninstallInstalledAgent_Default_KeepsSnapshot(t *testing.T) {
	store := NewMemL5Store()
	ctx := context.Background()
	if err := store.RegisterInstalledAgent(ctx, &InstalledAgent{
		UserID: "alice", AgentName: "weather", AgentVersion: "1.0",
	}); err != nil {
		t.Fatal(err)
	}
	res, err := store.UninstallInstalledAgent(ctx, "alice", "weather", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.UninstalledAt == nil {
		t.Error("UninstalledAt nil")
	}
	if res.SnapshotPath == "" {
		t.Error("SnapshotPath empty (default = keep data)")
	}
	// 还应该在 list(enabledOnly=false)
	all, _ := store.ListInstalledAgents(ctx, "alice", false)
	if len(all) != 1 {
		t.Errorf("all: got %d, want 1 (soft deleted)", len(all))
	}
}

// TestL5_UninstallInstalledAgent_Purge
func TestL5_UninstallInstalledAgent_Purge(t *testing.T) {
	store := NewMemL5Store()
	ctx := context.Background()
	if err := store.RegisterInstalledAgent(ctx, &InstalledAgent{
		UserID: "alice", AgentName: "weather", AgentVersion: "1.0",
	}); err != nil {
		t.Fatal(err)
	}
	_, err := store.UninstallInstalledAgent(ctx, "alice", "weather", true)
	if err != nil {
		t.Fatal(err)
	}
	all, _ := store.ListInstalledAgents(ctx, "alice", false)
	if len(all) != 0 {
		t.Errorf("all: got %d, want 0 (purged)", len(all))
	}
}

// TestL5_UninstallInstalledAgent_NotFound
func TestL5_UninstallInstalledAgent_NotFound(t *testing.T) {
	store := NewMemL5Store()
	_, err := store.UninstallInstalledAgent(context.Background(), "alice", "weather", false)
	if !errors.Is(err, ErrL5NotFound) {
		t.Errorf("err = %v, want ErrL5NotFound", err)
	}
}

// ============================================================
// user_skill_pool 测试
// ============================================================

// TestL5_AddUserSkill
func TestL5_AddUserSkill(t *testing.T) {
	store := NewMemL5Store()
	sk := &UserSkill{UserID: "alice", SkillName: "weather", SkillVersion: "1.0", Enabled: true}
	if err := store.AddUserSkill(context.Background(), sk); err != nil {
		t.Fatal(err)
	}
	if sk.ID == 0 || sk.CreatedAt.IsZero() {
		t.Error("ID / CreatedAt not set")
	}
}

// TestL5_AddUserSkill_Required
func TestL5_AddUserSkill_Required(t *testing.T) {
	store := NewMemL5Store()
	if err := store.AddUserSkill(context.Background(), &UserSkill{SkillName: "x"}); err == nil {
		t.Error("expected error for empty user_id")
	}
	if err := store.AddUserSkill(context.Background(), &UserSkill{UserID: "a"}); err == nil {
		t.Error("expected error for empty skill_name")
	}
}

// TestL5_ListUserSkills
func TestL5_ListUserSkills(t *testing.T) {
	store := NewMemL5Store()
	ctx := context.Background()
	store.AddUserSkill(ctx, &UserSkill{UserID: "alice", SkillName: "s1", Enabled: true})
	store.AddUserSkill(ctx, &UserSkill{UserID: "alice", SkillName: "s2", Enabled: false})
	all, _ := store.ListUserSkills(ctx, "alice", false)
	if len(all) != 2 {
		t.Errorf("all: got %d, want 2", len(all))
	}
	enabled, _ := store.ListUserSkills(ctx, "alice", true)
	if len(enabled) != 1 || enabled[0].SkillName != "s1" {
		t.Errorf("enabled: got %+v", enabled)
	}
}

// TestL5_RemoveUserSkill
func TestL5_RemoveUserSkill(t *testing.T) {
	store := NewMemL5Store()
	ctx := context.Background()
	store.AddUserSkill(ctx, &UserSkill{UserID: "alice", SkillName: "s1"})
	if err := store.RemoveUserSkill(ctx, "alice", "s1"); err != nil {
		t.Fatal(err)
	}
	all, _ := store.ListUserSkills(ctx, "alice", false)
	if len(all) != 0 {
		t.Errorf("got %d, want 0", len(all))
	}
	if err := store.RemoveUserSkill(ctx, "alice", "s1"); !errors.Is(err, ErrL5NotFound) {
		t.Errorf("err = %v, want ErrL5NotFound", err)
	}
}

// ============================================================
// agent_memory 测试
// ============================================================

// TestL5_SetAgentMemory_New
func TestL5_SetAgentMemory_New(t *testing.T) {
	store := NewMemL5Store()
	m := &AgentMemory{UserID: "alice", AgentName: "weather", Key: "city", Value: `"北京"`}
	if err := store.SetAgentMemory(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	if m.ID == 0 || m.CreatedAt.IsZero() || m.UpdatedAt.IsZero() {
		t.Error("ID/CreatedAt/UpdatedAt not set")
	}
}

// TestL5_SetAgentMemory_Update
func TestL5_SetAgentMemory_Update(t *testing.T) {
	store := NewMemL5Store()
	ctx := context.Background()
	store.SetAgentMemory(ctx, &AgentMemory{UserID: "alice", AgentName: "weather", Key: "city", Value: `"北京"`})
	m, _ := store.GetAgentMemory(ctx, "alice", "weather", "city")
	originalID := m.ID
	originalCreated := m.CreatedAt
	// update
	if err := store.SetAgentMemory(ctx, &AgentMemory{UserID: "alice", AgentName: "weather", Key: "city", Value: `"上海"`}); err != nil {
		t.Fatal(err)
	}
	m2, _ := store.GetAgentMemory(ctx, "alice", "weather", "city")
	if m2.ID != originalID {
		t.Errorf("ID changed: %d → %d", originalID, m2.ID)
	}
	if !m2.CreatedAt.Equal(originalCreated) {
		t.Error("CreatedAt changed (should be preserved)")
	}
	if m2.Value != `"上海"` {
		t.Errorf("Value = %q, want 上海", m2.Value)
	}
}

// TestL5_SetAgentMemory_Required
func TestL5_SetAgentMemory_Required(t *testing.T) {
	store := NewMemL5Store()
	if err := store.SetAgentMemory(context.Background(), &AgentMemory{AgentName: "a", Key: "k"}); err == nil {
		t.Error("expected error for empty user_id")
	}
	if err := store.SetAgentMemory(context.Background(), &AgentMemory{UserID: "a", Key: "k"}); err == nil {
		t.Error("expected error for empty agent_name")
	}
	if err := store.SetAgentMemory(context.Background(), &AgentMemory{UserID: "a", AgentName: "a"}); err == nil {
		t.Error("expected error for empty key")
	}
}

// TestL5_GetAgentMemory_NotFound
func TestL5_GetAgentMemory_NotFound(t *testing.T) {
	store := NewMemL5Store()
	_, err := store.GetAgentMemory(context.Background(), "alice", "weather", "city")
	if !errors.Is(err, ErrL5NotFound) {
		t.Errorf("err = %v, want ErrL5NotFound", err)
	}
}

// TestL5_ListAgentMemory
func TestL5_ListAgentMemory(t *testing.T) {
	store := NewMemL5Store()
	ctx := context.Background()
	store.SetAgentMemory(ctx, &AgentMemory{UserID: "alice", AgentName: "weather", Key: "city", Value: `"北京"`})
	store.SetAgentMemory(ctx, &AgentMemory{UserID: "alice", AgentName: "weather", Key: "unit", Value: `"celsius"`})
	all, _ := store.ListAgentMemory(ctx, "alice", "weather")
	if len(all) != 2 {
		t.Errorf("got %d, want 2", len(all))
	}
}

// TestL5_DeleteAgentMemory
func TestL5_DeleteAgentMemory(t *testing.T) {
	store := NewMemL5Store()
	ctx := context.Background()
	store.SetAgentMemory(ctx, &AgentMemory{UserID: "alice", AgentName: "weather", Key: "city", Value: `"北京"`})
	if err := store.DeleteAgentMemory(ctx, "alice", "weather", "city"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteAgentMemory(ctx, "alice", "weather", "city"); !errors.Is(err, ErrL5NotFound) {
		t.Errorf("err = %v, want ErrL5NotFound", err)
	}
}

// TestL5_Interface L5Store interface 断言(编译期)
func TestL5_Interface(t *testing.T) {
	var _ L5Store = (*MemL5Store)(nil)
	var _ L5Store = NewMemL5Store()
}