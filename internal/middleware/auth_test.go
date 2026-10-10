package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"watchAlert/config"
	"watchAlert/internal/ctx"
	"watchAlert/internal/health"
	"watchAlert/internal/models"
	"watchAlert/internal/repo"
	"watchAlert/pkg/tools"

	"github.com/dgrijalva/jwt-go"
	"github.com/ecodeclub/ekit"
	"github.com/ecodeclub/ginx/gctx"
	"github.com/ecodeclub/ginx/session"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============ 手写 mock（ginx session.Provider / session.Session） ============

type fakeSession struct {
	claims session.Claims
	getErr error // Get(ctx, "uid") 返回的错误；nil=存活，redis.Nil=已销毁，其它=连接失败
}

func (f *fakeSession) Set(context.Context, string, any) error { return nil }
func (f *fakeSession) Del(context.Context, string) error      { return nil }
func (f *fakeSession) Destroy(context.Context) error          { return nil }
func (f *fakeSession) Claims() session.Claims                 { return f.claims }
func (f *fakeSession) Get(context.Context, string) ekit.AnyValue {
	if f.getErr != nil {
		return ekit.AnyValue{Err: f.getErr}
	}
	return ekit.AnyValue{Val: "1"}
}

type fakeProvider struct {
	sess *fakeSession
	err  error // Get 返回错误时用于 401 分支
}

func (p *fakeProvider) NewSession(*gctx.Context, int64, map[string]string, map[string]any) (session.Session, error) {
	return p.sess, nil
}
func (p *fakeProvider) Get(*gctx.Context) (session.Session, error) {
	if p.err != nil {
		return nil, p.err
	}
	return p.sess, nil
}
func (p *fakeProvider) Destroy(*gctx.Context) error                      { return nil }
func (p *fakeProvider) UpdateClaims(*gctx.Context, session.Claims) error { return nil }
func (p *fakeProvider) RenewAccessToken(*gctx.Context) error             { return nil }

// ============ 测试基础设施 ============

// setupAuthTest 构建 gin engine（/read + /write 路由挂 Auth），返回 engine 与 repoEntry。
func setupAuthTest(t *testing.T, provider session.Provider) (*gin.Engine, repo.InterEntryRepo) {
	t.Helper()
	config.Application.Database.Type = "sqlite"
	config.Application.Database.Path = filepath.Join(t.TempDir(), "auth_test.db")
	config.Application.Server.Mode = "test"

	repoEntry := repo.NewRepoEntry()
	require.NotNil(t, repoEntry, "repo 初始化失败")
	ctx.NewContext(context.Background(), repoEntry, nil)

	sessionCache = NewSessionCache(1024, 30*time.Second)
	InitSession(provider)
	health.ResetForTests()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/read", Auth(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.POST("/write", Auth(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	t.Cleanup(func() {
		if sqlDB, err := repoEntry.DB().DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return r, repoEntry
}

func doAuthRequest(t *testing.T, r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ============ 测试用例 ============

func TestAuth_Unauthorized_NoSession(t *testing.T) {
	r, _ := setupAuthTest(t, &fakeProvider{err: errors.New("token invalid")})

	w := doAuthRequest(t, r, http.MethodGet, "/read")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), `"code":1401`)
}

func TestAuth_Forbidden_NoRoleBinding(t *testing.T) {
	p := &fakeProvider{sess: &fakeSession{claims: session.Claims{Uid: 999, SSID: "ssid-x"}}}
	r, _ := setupAuthTest(t, p)

	w := doAuthRequest(t, r, http.MethodGet, "/read")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), `"code":1403`)
}

func TestAuth_Success_WithRoleBinding(t *testing.T) {
	p := &fakeProvider{sess: &fakeSession{claims: session.Claims{Uid: 1001, SSID: "ssid-ok"}}}
	r, repoEntry := setupAuthTest(t, p)

	_, err := repoEntry.RoleBinding().Upsert(models.RoleBinding{EiamUid: 1001, Role: models.RoleAdmin})
	require.NoError(t, err)

	w := doAuthRequest(t, r, http.MethodGet, "/read")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_Unavailable_RedisDown_Write(t *testing.T) {
	p := &fakeProvider{sess: &fakeSession{
		claims: session.Claims{Uid: 1001, SSID: "ssid-down"},
		getErr: errors.New("connection refused"),
	}}
	r, _ := setupAuthTest(t, p)

	w := doAuthRequest(t, r, http.MethodPost, "/write")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), `"code":1503`)
}

func TestAuth_SessionCache_ReadFallback(t *testing.T) {
	p := &fakeProvider{sess: &fakeSession{claims: session.Claims{Uid: 1001, SSID: "ssid-cache"}}}
	r, repoEntry := setupAuthTest(t, p)

	_, err := repoEntry.RoleBinding().Upsert(models.RoleBinding{EiamUid: 1001, Role: models.RoleAdmin})
	require.NoError(t, err)

	// 第一次：Redis 存活 → 200，写入缓存
	w1 := doAuthRequest(t, r, http.MethodGet, "/read")
	assert.Equal(t, http.StatusOK, w1.Code)

	// 模拟 Redis 抖动：读接口 + 缓存命中 → 200
	p.sess.getErr = errors.New("connection refused")
	w2 := doAuthRequest(t, r, http.MethodGet, "/read")
	assert.Equal(t, http.StatusOK, w2.Code, "读接口 + 缓存命中应放行")

	// 写接口 + Redis 抖动 → 503（不查缓存）
	w3 := doAuthRequest(t, r, http.MethodPost, "/write")
	assert.Equal(t, http.StatusServiceUnavailable, w3.Code)
}

func TestAuth_Unauthorized_SessionDestroyed(t *testing.T) {
	p := &fakeProvider{sess: &fakeSession{
		claims: session.Claims{Uid: 1001, SSID: "ssid-destroyed"},
		getErr: redis.Nil, // 会话已销毁（logout）
	}}
	r, _ := setupAuthTest(t, p)

	w := doAuthRequest(t, r, http.MethodGet, "/read")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), `"code":1401`)
}

// TestAuth_LegacyJWT_ReadTolerate_WriteReject 过渡期旧本地 JWT 分派：
// 读接口容忍（验签只读副本）、写接口拒绝 403（强制走共享会话）。
func TestAuth_LegacyJWT_ReadTolerate_WriteReject(t *testing.T) {
	config.Application.Jwt.VerifyOnlyKey = "legacy-test-key"

	// sessionProvider 一律拒绝（模拟旧 token 非共享会话）
	p := &fakeProvider{err: errors.New("not a shared session")}
	r, _ := setupAuthTest(t, p)

	legacyToken := generateLegacyJWT(t, "uid-legacy")

	// 读接口 → 容忍放行
	req := httptest.NewRequest(http.MethodGet, "/read", nil)
	req.Header.Set("Authorization", legacyToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "旧 JWT 读接口应容忍至过期")

	// 写接口 → 403 拒绝
	req2 := httptest.NewRequest(http.MethodPost, "/write", nil)
	req2.Header.Set("Authorization", legacyToken)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusForbidden, w2.Code, "旧 JWT 写接口应拒绝")
	assert.Contains(t, w2.Body.String(), `"code":1403`)
}

// generateLegacyJWT 用验签副本在测试内生成旧本地 JWT（GenerateToken 已移除，测试内手工签发）。
func generateLegacyJWT(t *testing.T, uid string) string {
	t.Helper()
	claims := tools.JwtCustomClaims{
		ID:   uid,
		Name: "legacy-user",
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
			IssuedAt:  time.Now().Unix(),
			Issuer:    "WatchAlert",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte(config.Application.Jwt.VerifyOnlyKey))
	require.NoError(t, err)
	return "bearer " + s
}

// TestAuth_Write503_WhenDegraded 降级状态（write_disabled）下写接口统一 503，读接口不受影响。
func TestAuth_Write503_WhenDegraded(t *testing.T) {
	// 有效会话 + 有效角色绑定
	p := &fakeProvider{sess: &fakeSession{claims: session.Claims{Uid: 1001, SSID: "ssid-ok"}}}
	r, repoEntry := setupAuthTest(t, p)
	_, err := repoEntry.RoleBinding().Upsert(models.RoleBinding{EiamUid: 1001, Role: models.RoleAdmin})
	require.NoError(t, err)

	// 未降级：写接口正常（无角色、200 会因角色/会话走通？这里会话有效角色绑定存在 → 200）
	w0 := doAuthRequest(t, r, http.MethodPost, "/write")
	assert.Equal(t, http.StatusOK, w0.Code)

	// 探针 B 连续 3 失败 → degraded（write_disabled）
	health.RecordProbeB(false)
	health.RecordProbeB(false)
	health.RecordProbeB(false)
	assert.True(t, health.WriteDisabled())

	// 写接口 → 503（降级早退）
	w := doAuthRequest(t, r, http.MethodPost, "/write")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), `"code":1503`)

	// 读接口 → 仍走共享会话 → 200
	w2 := doAuthRequest(t, r, http.MethodGet, "/read")
	assert.Equal(t, http.StatusOK, w2.Code, "读接口不受降级影响")
}
