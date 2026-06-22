package service

import (
	"context"
	"sync"
	"testing"
)

// TestMemStore_GetSet:基础 Get/Set 测试
func TestMemStore_GetSet(t *testing.T) {
	m := NewMemStore()
	ctx := context.Background()

	p := &Profile{
		UserID:     "u1",
		Role:       "doctor",
		Department: "healthcare",
	}

	if err := m.Set(ctx, "tenant-A", p); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, found, err := m.Get(ctx, "tenant-A", "u1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("Get 应 found")
	}
	if got.UserID != "u1" {
		t.Errorf("UserID: got %q, want %q", got.UserID, "u1")
	}
	if got.Role != "doctor" {
		t.Errorf("Role: got %q, want %q", got.Role, "doctor")
	}
}

// TestMemStore_Get_NotFound:找不到应返 (nil, false, nil)
func TestMemStore_Get_NotFound(t *testing.T) {
	m := NewMemStore()
	got, found, err := m.Get(context.Background(), "tenant-A", "nonexistent")
	if err != nil {
		t.Errorf("err should be nil, got %v", err)
	}
	if found {
		t.Error("found should be false")
	}
	if got != nil {
		t.Errorf("profile should be nil, got %+v", got)
	}
}

// TestMemStore_Get_EmptyUserID:user_id 空应返 error
func TestMemStore_Get_EmptyUserID(t *testing.T) {
	m := NewMemStore()
	_, _, err := m.Get(context.Background(), "tenant-A", "")
	if err == nil {
		t.Error("empty user_id 应返 error")
	}
}

// TestMemStore_Get_ReturnsClone:修改返回值不影响 cache
func TestMemStore_Get_ReturnsClone(t *testing.T) {
	m := NewMemStore()
	m.Set(context.Background(), "tenant-A", &Profile{
		UserID:          "u1",
		Role:            "doctor",
		PreferredSkills: []string{"diagnosis"},
	})

	got, _, _ := m.Get(context.Background(), "tenant-A", "u1")
	got.Role = "hacker"
	got.PreferredSkills[0] = "malicious"

	got2, _, _ := m.Get(context.Background(), "tenant-A", "u1")
	if got2.Role != "doctor" {
		t.Errorf("cache 被污染:Role=%q,应仍为 doctor", got2.Role)
	}
	if got2.PreferredSkills[0] != "diagnosis" {
		t.Errorf("cache 被污染:PreferredSkills[0]=%q,应仍为 diagnosis", got2.PreferredSkills[0])
	}
}

// TestMemStore_TenantIsolation:不同 tenant 同 user_id 应独立
func TestMemStore_TenantIsolation(t *testing.T) {
	m := NewMemStore()
	ctx := context.Background()

	m.Set(ctx, "tenant-A", &Profile{UserID: "u1", Role: "doctor"})
	m.Set(ctx, "tenant-B", &Profile{UserID: "u1", Role: "developer"})

	gotA, _, _ := m.Get(ctx, "tenant-A", "u1")
	gotB, _, _ := m.Get(ctx, "tenant-B", "u1")

	if gotA.Role != "doctor" {
		t.Errorf("tenant-A role: got %q, want doctor", gotA.Role)
	}
	if gotB.Role != "developer" {
		t.Errorf("tenant-B role: got %q, want developer", gotB.Role)
	}
}

// TestMemStore_DefaultTenant:tenant_id 空应走 "default" tenant
func TestMemStore_DefaultTenant(t *testing.T) {
	m := NewMemStore()
	m.Set(context.Background(), "", &Profile{UserID: "u1", Role: "doctor"})

	got, found, _ := m.Get(context.Background(), "", "u1")
	if !found {
		t.Error("default tenant 应 found")
	}
	if got.Role != "doctor" {
		t.Errorf("role: got %q", got.Role)
	}
}

// TestMemStore_Delete:Delete 应返 found 标志
func TestMemStore_Delete(t *testing.T) {
	m := NewMemStore()
	ctx := context.Background()

	m.Set(ctx, "tenant-A", &Profile{UserID: "u1", Role: "doctor"})

	// 第一次删应 found
	deleted, err := m.Delete(ctx, "tenant-A", "u1")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !deleted {
		t.Error("Delete 应返 true")
	}

	// 第二次删应 not found
	deleted, err = m.Delete(ctx, "tenant-A", "u1")
	if err != nil {
		t.Fatalf("Delete 2nd: %v", err)
	}
	if deleted {
		t.Error("第二次 Delete 应返 false")
	}
}

// TestMemStore_Delete_EmptyUserID:user_id 空应返 error
func TestMemStore_Delete_EmptyUserID(t *testing.T) {
	m := NewMemStore()
	_, err := m.Delete(context.Background(), "tenant-A", "")
	if err == nil {
		t.Error("empty user_id 应返 error")
	}
}

// TestMemStore_Concurrent:并发读写不 panic
func TestMemStore_Concurrent(t *testing.T) {
	m := NewMemStore()
	ctx := context.Background()

	m.Set(ctx, "tenant-A", &Profile{UserID: "u1", Role: "doctor"})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _, _ = m.Get(ctx, "tenant-A", "u1")
			}
		}()
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = m.Set(ctx, "tenant-A", &Profile{UserID: "u1", Role: "doctor"})
			}
		}(i)
	}
	wg.Wait()
}

// TestMemStore_Health:Health 应返 nil
func TestMemStore_Health(t *testing.T) {
	m := NewMemStore()
	if err := m.Health(context.Background()); err != nil {
		t.Errorf("Health 应返 nil,实际 %v", err)
	}
}
