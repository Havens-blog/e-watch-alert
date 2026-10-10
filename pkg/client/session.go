package client

import (
	"fmt"
	"time"

	"watchAlert/config"

	"github.com/ecodeclub/ginx/session"
	"github.com/ecodeclub/ginx/session/cookie"
	"github.com/ecodeclub/ginx/session/header"
	"github.com/ecodeclub/ginx/session/mixin"
	ginRedis "github.com/ecodeclub/ginx/session/redis"
	"github.com/redis/go-redis/v9"
)

// sessionExpiration 与 eiam ioc/session.go 硬编码一致（30 天）。
// WatchAlert 侧必须传同一常量值，不自定（tech-design §3.1）。
const sessionExpiration = 30 * 24 * time.Hour

// InitSession 初始化共享会话 provider（复用 eiam 同款 ginx/session）。
// 会话密钥 / cookie 名 / TTL 与 eiam 一致是互读会话的前提。
// 返回的 provider 用独立 go-redis/v9 客户端（ginx 依赖 v9，与业务侧 go-redis v6 隔离）。
func InitSession() session.Provider {
	cfg := config.Application.Session
	if cfg.SessionEncryptedKey == "" {
		panic("session.session_encrypted_key 未配置：必须与 eiam 一致的非空共享会话签名密钥")
	}
	if cfg.Cookie.Name == "" {
		panic("session.cookie.name 未配置：必须与 eiam 一致（ecmdb-token-key）")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", config.Application.Redis.Host, config.Application.Redis.Port),
		Password: config.Application.Redis.Pass,
		DB:       config.Application.Redis.Database,
	})

	provider := ginRedis.NewSessionProvider(rdb, cfg.SessionEncryptedKey, sessionExpiration)

	headerCarrier := header.NewTokenCarrier()
	if cfg.TokenCarrier == "token" {
		provider.TokenCarrier = headerCarrier
		return provider
	}

	// 默认 cookie 主 + Authorization 头兜底（与 eiam mixin 组合一致）
	provider.TokenCarrier = mixin.NewTokenCarrier(newCookieTokenCarrier(cfg.Cookie), headerCarrier)
	return provider
}

func newCookieTokenCarrier(cfg config.SessionCookie) *cookie.TokenCarrier {
	secure := true
	if cfg.Secure != nil {
		secure = *cfg.Secure
	}
	httpOnly := true
	if cfg.HttpOnly != nil {
		httpOnly = *cfg.HttpOnly
	}
	return &cookie.TokenCarrier{
		MaxAge:   int(sessionExpiration.Seconds()),
		Name:     cfg.Name,
		Path:     "/",
		Secure:   secure,
		HttpOnly: httpOnly,
		Domain:   cfg.Domain,
	}
}
