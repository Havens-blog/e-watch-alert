package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"watchAlert/config"

	"github.com/alicebob/miniredis/v2"
	"github.com/ecodeclub/ginx/gctx"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitSession_HeaderCarrier_RoundTrip 验证共享会话 provider 能签发并读回会话 Claims（uid/username）。
func TestInitSession_HeaderCarrier_RoundTrip(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	// 指向 miniredis，使用 header 载体（token 走 X-Access-Token 响应头 + Authorization 请求头）
	setSessionConfig(mr, "token")

	sp := InitSession()
	require.NotNil(t, sp)

	// Step 1: NewSession 签发会话
	rec := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(rec)
	g1 := &gctx.Context{Context: c1}
	sess, err := sp.NewSession(g1, 123, map[string]string{"username": "alice"}, nil)
	require.NoError(t, err)
	require.NotNil(t, sess)
	assert.Equal(t, int64(123), sess.Claims().Uid)
	assert.Equal(t, "alice", sess.Claims().Data["username"])

	// token 写入响应头 X-Access-Token
	token := rec.Header().Get("X-Access-Token")
	require.NotEmpty(t, token)

	// Step 2: Get 用 Authorization: Bearer 读回会话
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(rec2)
	c2.Request = req
	g2 := &gctx.Context{Context: c2}
	got, err := sp.Get(g2)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, int64(123), got.Claims().Uid)
	assert.Equal(t, "alice", got.Claims().Data["username"])
}

// TestInitSession_CookieCarrier_RoundTrip 验证默认 cookie 载体（与 eiam 一致）能往返读写会话。
func TestInitSession_CookieCarrier_RoundTrip(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	setSessionConfig(mr, "cookie")

	sp := InitSession()
	require.NotNil(t, sp)

	rec := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(rec)
	g1 := &gctx.Context{Context: c1}
	sess, err := sp.NewSession(g1, 456, map[string]string{"username": "bob"}, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(456), sess.Claims().Uid)

	// cookie 载体：token 写入 Set-Cookie
	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
	var tokenCookie string
	for _, c := range cookies {
		if c.Name == "ecmdb-token-key" {
			tokenCookie = c.Value
		}
	}
	require.NotEmpty(t, tokenCookie, "Set-Cookie 应含 ecmdb-token-key")

	// 读回：请求带同款 cookie
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "ecmdb-token-key", Value: tokenCookie})
	rec2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(rec2)
	c2.Request = req
	g2 := &gctx.Context{Context: c2}
	got, err := sp.Get(g2)
	require.NoError(t, err)
	assert.Equal(t, int64(456), got.Claims().Uid)
	assert.Equal(t, "bob", got.Claims().Data["username"])
}

// TestNewCookieTokenCarrier 验证 cookie 载体配置与 eiam 约定一致。
func TestNewCookieTokenCarrier(t *testing.T) {
	secure := false
	httpOnly := true
	cfg := config.SessionCookie{
		Domain:   "localhost",
		Name:     "ecmdb-token-key",
		Secure:   &secure,
		HttpOnly: &httpOnly,
	}
	carrier := newCookieTokenCarrier(cfg)
	assert.Equal(t, "ecmdb-token-key", carrier.Name)
	assert.Equal(t, "localhost", carrier.Domain)
	assert.Equal(t, "/", carrier.Path)
	assert.False(t, carrier.Secure)
	assert.True(t, carrier.HttpOnly)
	assert.Equal(t, int(sessionExpiration.Seconds()), carrier.MaxAge)
}

// TestInitSession_MissingKey_Panics 验证密钥/cookie 名缺失时快速失败（防误配）。
func TestInitSession_MissingKey_Panics(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	config.Application.Redis.Host = mr.Host()
	config.Application.Redis.Port = mr.Port()
	config.Application.Session.SessionEncryptedKey = ""
	config.Application.Session.Cookie.Name = "ecmdb-token-key"

	assert.Panics(t, func() { InitSession() })
}

// setSessionConfig 指向 miniredis 并设置会话配置（测试隔离用）。
func setSessionConfig(mr *miniredis.Miniredis, tokenCarrier string) {
	config.Application.Redis.Host = mr.Host()
	config.Application.Redis.Port = mr.Port()
	config.Application.Redis.Database = 0
	config.Application.Session.SessionEncryptedKey = "test-shared-session-key"
	config.Application.Session.TokenCarrier = tokenCarrier
	secure := false
	httpOnly := true
	config.Application.Session.Cookie = config.SessionCookie{
		Domain:   "localhost",
		Name:     "ecmdb-token-key",
		Secure:   &secure,
		HttpOnly: &httpOnly,
	}
}
