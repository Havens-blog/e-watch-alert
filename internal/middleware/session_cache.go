package middleware

import (
	"container/list"
	"sync"
	"time"

	"github.com/ecodeclub/ginx/session"
)

// sessionCacheEntry 会话缓存条目（缓存值 = 解密后的 session.Claims 快照，不含原始密文）。
type sessionCacheEntry struct {
	ssid     string
	claims   session.Claims
	expireAt time.Time
}

// SessionCache 进程内会话缓存兜底层（tech-design §2.4）：
// 仅在 Redis 读失败时作为已认证读请求的兜底，吸收 Redis 秒级抖动。
// TTL + LRU 双重淘汰，上限 maxSize（默认 1024）。
type SessionCache struct {
	mu      sync.Mutex
	ll      *list.List
	items   map[string]*list.Element
	maxSize int
	ttl     time.Duration
}

func NewSessionCache(maxSize int, ttl time.Duration) *SessionCache {
	return &SessionCache{
		ll:      list.New(),
		items:   make(map[string]*list.Element),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get 命中返回缓存 claims；未命中或已过期返回 false。命中时刷新 LRU 位置。
func (sc *SessionCache) Get(ssid string) (session.Claims, bool) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	el, ok := sc.items[ssid]
	if !ok {
		return session.Claims{}, false
	}
	e := el.Value.(*sessionCacheEntry)
	if time.Now().After(e.expireAt) {
		sc.remove(el)
		return session.Claims{}, false
	}
	sc.ll.MoveToFront(el)
	return e.claims, true
}

// Set 写入/刷新缓存；达到上限时驱逐最久未用（LRU 队尾）。
func (sc *SessionCache) Set(ssid string, claims session.Claims) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	now := time.Now()
	if el, ok := sc.items[ssid]; ok {
		e := el.Value.(*sessionCacheEntry)
		e.claims = claims
		e.expireAt = now.Add(sc.ttl)
		sc.ll.MoveToFront(el)
		return
	}

	if sc.ll.Len() >= sc.maxSize {
		if back := sc.ll.Back(); back != nil {
			sc.remove(back)
		}
	}

	el := sc.ll.PushFront(&sessionCacheEntry{ssid: ssid, claims: claims, expireAt: now.Add(sc.ttl)})
	sc.items[ssid] = el
}

func (sc *SessionCache) remove(el *list.Element) {
	sc.ll.Remove(el)
	delete(sc.items, el.Value.(*sessionCacheEntry).ssid)
}

// Len 返回当前条目数（测试用）。
func (sc *SessionCache) Len() int {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.ll.Len()
}
