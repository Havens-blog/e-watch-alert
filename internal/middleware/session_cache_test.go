package middleware

import (
	"testing"
	"time"

	"github.com/ecodeclub/ginx/session"
	"github.com/stretchr/testify/assert"
)

func TestSessionCache_SetGet(t *testing.T) {
	cache := NewSessionCache(1024, 30*time.Second)
	claims := session.Claims{Uid: 1001, SSID: "ssid-1"}
	cache.Set("ssid-1", claims)

	got, ok := cache.Get("ssid-1")
	assert.True(t, ok)
	assert.Equal(t, int64(1001), got.Uid)
}

func TestSessionCache_Miss(t *testing.T) {
	cache := NewSessionCache(1024, 30*time.Second)
	_, ok := cache.Get("nonexistent")
	assert.False(t, ok)
}

func TestSessionCache_TTL(t *testing.T) {
	cache := NewSessionCache(1024, 50*time.Millisecond)
	cache.Set("ssid-1", session.Claims{Uid: 1, SSID: "ssid-1"})

	_, ok := cache.Get("ssid-1")
	assert.True(t, ok, "TTL 内应命中")

	time.Sleep(60 * time.Millisecond)
	_, ok = cache.Get("ssid-1")
	assert.False(t, ok, "TTL 过期应驱逐")
	assert.Equal(t, 0, cache.Len())
}

func TestSessionCache_LRU(t *testing.T) {
	cache := NewSessionCache(2, 30*time.Second)
	cache.Set("a", session.Claims{SSID: "a"})
	cache.Set("b", session.Claims{SSID: "b"})

	// 访问 a，使其成为最近使用
	_, _ = cache.Get("a")

	// 写入 c，应驱逐最久未用的 b
	cache.Set("c", session.Claims{SSID: "c"})

	_, okA := cache.Get("a")
	assert.True(t, okA, "a 应仍在缓存")
	_, okB := cache.Get("b")
	assert.False(t, okB, "b 应被 LRU 驱逐")
	_, okC := cache.Get("c")
	assert.True(t, okC, "c 应在缓存")
	assert.Equal(t, 2, cache.Len())
}
