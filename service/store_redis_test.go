package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisTestAddr 测试用 Redis 地址(连 43.134.126.126:6379,db 2)
// env WAU_PROFILE_REDIS_ADDR 可覆盖
func redisTestAddr() string {
	if v := os.Getenv("WAU_PROFILE_REDIS_ADDR"); v != "" {
		return v
	}
	return "43.134.126.126:6379"
}

func redisTestPassword() string {
	return os.Getenv("WAU_TEST_REDIS_PASSWORD")
}

// newTestRedisClient 建测试用 Redis client(连真 Redis,db 15 避免污染 db 2)
//
// 如果 Redis 不可用 → t.Skip(跟 wau-scheduler 模式一致)
func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := redisTestAddr()
	password := redisTestPassword()
	client := redis.NewClient(&redis.Options{
		Addr:        addr,
		Password:    password,
		DB:          15, // 单独用 db 15 做测试,跟 db 2 完全隔离
		DialTimeout: 2 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Skipf("Redis not available at %s db=15: %v", addr, err)
	}
	// 清理 db 15 test 残留
	_ = client.FlushDB(ctx).Err()
	t.Cleanup(func() {
		_ = client.FlushDB(context.Background()).Err()
		_ = client.Close()
	})
	return client
}

// TestRedisStore_GetSet:基础 Get/Set
func TestRedisStore_GetSet(t *testing.T) {
	client := newTestRedisClient(t)
	store := NewRedisStore(client, "wau:profile:v1:test:", []string{"default", "tenant-A"}, 0)
	ctx := context.Background()

	p := &Profile{
		UserID:     "u1",
		Role:       "doctor",
		Department: "healthcare",
	}
	if err := store.Set(ctx, "tenant-A", p); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, found, err := store.Get(ctx, "tenant-A", "u1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("Get 应 found")
	}
	if got.UserID != "u1" {
		t.Errorf("UserID: got %q, want u1", got.UserID)
	}
	if got.Role != "doctor" {
		t.Errorf("Role: got %q, want doctor", got.Role)
	}
}

// TestRedisStore_Get_NotFound:redis.Nil 应视作 not_found(不返 error)
func TestRedisStore_Get_NotFound(t *testing.T) {
	client := newTestRedisClient(t)
	store := NewRedisStore(client, "wau:profile:v1:test:", []string{"default"}, 0)

	got, found, err := store.Get(context.Background(), "default", "nonexistent")
	if err != nil {
		t.Errorf("not found should not return err, got %v", err)
	}
	if found {
		t.Error("found should be false")
	}
	if got != nil {
		t.Error("profile should be nil")
	}
}

// TestRedisStore_Delete:Delete 应返 found 标志
func TestRedisStore_Delete(t *testing.T) {
	client := newTestRedisClient(t)
	store := NewRedisStore(client, "wau:profile:v1:test:", []string{"default"}, 0)
	ctx := context.Background()

	// 先写入
	_ = store.Set(ctx, "default", &Profile{UserID: "u1", Role: "doctor"})

	// 第一次删应 found
	deleted, err := store.Delete(ctx, "default", "u1")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !deleted {
		t.Error("Delete 应返 true")
	}

	// 第二次删应 not found
	deleted, _ = store.Delete(ctx, "default", "u1")
	if deleted {
		t.Error("第二次 Delete 应返 false")
	}
}

// TestRedisStore_TenantReject:不在白名单的 tenant 应返 error
func TestRedisStore_TenantReject(t *testing.T) {
	client := newTestRedisClient(t)
	store := NewRedisStore(client, "wau:profile:v1:test:", []string{"default"}, 0)
	ctx := context.Background()

	// Get 不允许的 tenant
	_, _, err := store.Get(ctx, "forbidden", "u1")
	if err == nil {
		t.Error("forbidden tenant Get 应返 error")
	}
	if !contains(err.Error(), "not allowed") {
		t.Errorf("error 应含 'not allowed',实际 %q", err.Error())
	}

	// Set 不允许的 tenant
	err = store.Set(ctx, "forbidden", &Profile{UserID: "u1", Role: "doctor"})
	if err == nil {
		t.Error("forbidden tenant Set 应返 error")
	}

	// Delete 不允许的 tenant
	_, err = store.Delete(ctx, "forbidden", "u1")
	if err == nil {
		t.Error("forbidden tenant Delete 应返 error")
	}
}

// TestRedisStore_DefaultTenant:空 tenant 应走 "default"
func TestRedisStore_DefaultTenant(t *testing.T) {
	client := newTestRedisClient(t)
	store := NewRedisStore(client, "wau:profile:v1:test:", []string{"default"}, 0)

	// 空 tenant Set 应成功
	err := store.Set(context.Background(), "", &Profile{UserID: "u1", Role: "doctor"})
	if err != nil {
		t.Errorf("空 tenant Set 应成功,实际 %v", err)
	}

	// 空 tenant Get 应能拉
	got, found, _ := store.Get(context.Background(), "", "u1")
	if !found || got == nil {
		t.Error("空 tenant Get 应 found")
	}
}

// TestRedisStore_TenantIsolation:不同 tenant 同 user_id 应独立(key 不冲突)
func TestRedisStore_TenantIsolation(t *testing.T) {
	client := newTestRedisClient(t)
	store := NewRedisStore(client, "wau:profile:v1:test:", []string{"tenant-A", "tenant-B"}, 0)
	ctx := context.Background()

	_ = store.Set(ctx, "tenant-A", &Profile{UserID: "u1", Role: "doctor"})
	_ = store.Set(ctx, "tenant-B", &Profile{UserID: "u1", Role: "developer"})

	gotA, _, _ := store.Get(ctx, "tenant-A", "u1")
	gotB, _, _ := store.Get(ctx, "tenant-B", "u1")

	if gotA.Role != "doctor" {
		t.Errorf("tenant-A role: got %q, want doctor", gotA.Role)
	}
	if gotB.Role != "developer" {
		t.Errorf("tenant-B role: got %q, want developer", gotB.Role)
	}
}

// TestRedisStore_Health:Health 应返 nil(PING 成功)
func TestRedisStore_Health(t *testing.T) {
	client := newTestRedisClient(t)
	store := NewRedisStore(client, "wau:profile:v1:test:", []string{"default"}, 0)
	if err := store.Health(context.Background()); err != nil {
		t.Errorf("Health 应返 nil,实际 %v", err)
	}
}

// TestRedisStore_UpdateAllowedTenants:动态更新白名单
func TestRedisStore_UpdateAllowedTenants(t *testing.T) {
	client := newTestRedisClient(t)
	store := NewRedisStore(client, "wau:profile:v1:test:", []string{"default"}, 0)
	ctx := context.Background()

	// 初始白名单只有 default
	_, _, err := store.Get(ctx, "new-tenant", "u1")
	if err == nil {
		t.Error("new-tenant 不在白名单,Get 应返 error")
	}

	// 动态加白名单
	store.UpdateAllowedTenants([]string{"default", "new-tenant"})

	// 现在 new-tenant 应能 Get
	_, _, err = store.Get(ctx, "new-tenant", "u1")
	if err != nil {
		t.Errorf("UpdateAllowedTenants 后 Get 应成功,实际 %v", err)
	}
}

// TestRedisStore_Get_ReturnsClone:返 deep clone(防 caller 改污染)
func TestRedisStore_Get_ReturnsClone(t *testing.T) {
	client := newTestRedisClient(t)
	store := NewRedisStore(client, "wau:profile:v1:test:", []string{"default"}, 0)
	ctx := context.Background()

	_ = store.Set(ctx, "default", &Profile{
		UserID:          "u1",
		Role:            "doctor",
		PreferredSkills: []string{"diagnosis"},
	})

	got, _, _ := store.Get(ctx, "default", "u1")
	got.Role = "hacker"
	got.PreferredSkills[0] = "malicious"

	got2, _, _ := store.Get(ctx, "default", "u1")
	if got2.Role != "doctor" {
		t.Errorf("cache 被污染:Role=%q,应仍为 doctor", got2.Role)
	}
}

// TestRedisStore_TTL:TTL > 0 时 Set 应带过期
func TestRedisStore_TTL(t *testing.T) {
	client := newTestRedisClient(t)
	store := NewRedisStore(client, "wau:profile:v1:test:", []string{"default"}, 1*time.Second)
	ctx := context.Background()

	_ = store.Set(ctx, "default", &Profile{UserID: "u1", Role: "doctor"})

	// 立即 Get 应 hit
	_, found, _ := store.Get(ctx, "default", "u1")
	if !found {
		t.Fatal("Set 后应 hit")
	}

	// 等 1.5s 后应 miss
	time.Sleep(1500 * time.Millisecond)
	_, found, _ = store.Get(ctx, "default", "u1")
	if found {
		t.Error("TTL 过期后应 miss")
	}
}

// 私有 helper(跟 wau-scheduler 类似,避免 import strings)
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
