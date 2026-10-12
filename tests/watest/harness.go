// Package watest provides shared test infrastructure for WatchAlert API functional tests.
// It exports mock session providers, gin engine setup, and HTTP request/assertion helpers
// so that per-journey test files can focus on contract assertions.
package watest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"watchAlert/config"
	"watchAlert/internal/ctx"
	"watchAlert/internal/health"
	"watchAlert/internal/middleware"
	"watchAlert/internal/models"
	"watchAlert/internal/repo"
	routersv1 "watchAlert/internal/routers/v1"
	"watchAlert/internal/services"
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

// ============ Mock session provider (exported versions of middleware fakeProvider) ============

// FakeSession implements session.Session for testing.
type FakeSession struct {
	ClaimsData session.Claims
	GetErr     error // Get(ctx, "uid") return: nil=alive, redis.Nil=destroyed, other=conn failure
}

func (f *FakeSession) Set(context.Context, string, any) error { return nil }
func (f *FakeSession) Del(context.Context, string) error      { return nil }
func (f *FakeSession) Destroy(context.Context) error          { return nil }
func (f *FakeSession) Claims() session.Claims                 { return f.ClaimsData }
func (f *FakeSession) Get(ctx context.Context, key string) ekit.AnyValue {
	if f.GetErr != nil {
		return ekit.AnyValue{Err: f.GetErr}
	}
	return ekit.AnyValue{Val: "1"}
}

// FakeProvider implements session.Provider for testing.
type FakeProvider struct {
	Sess *FakeSession
	Err  error // non-nil → Get returns error (simulates no/invalid session)
}

func (p *FakeProvider) NewSession(*gctx.Context, int64, map[string]string, map[string]any) (session.Session, error) {
	return p.Sess, nil
}
func (p *FakeProvider) Get(*gctx.Context) (session.Session, error) {
	if p.Err != nil {
		return nil, p.Err
	}
	return p.Sess, nil
}
func (p *FakeProvider) Destroy(*gctx.Context) error                      { return nil }
func (p *FakeProvider) UpdateClaims(*gctx.Context, session.Claims) error { return nil }
func (p *FakeProvider) RenewAccessToken(*gctx.Context) error             { return nil }
func (p *FakeProvider) SetSession(s *FakeSession)                        { p.Sess = s }

// ============ Test Harness ============

// Harness bundles test environment state: gin engine, repo, and session provider.
type Harness struct {
	Engine   *gin.Engine
	Repo     repo.InterEntryRepo
	Provider *FakeProvider
}

// NewHarness creates a test harness with sqlite DB, mock session provider, and all routes registered.
// The provider controls auth behavior: nil provider → 503; provider with Err → 401; etc.
func NewHarness(t *testing.T, provider *FakeProvider) *Harness {
	t.Helper()

	config.Application.Database.Type = "sqlite"
	config.Application.Database.Path = filepath.Join(t.TempDir(), "watest.db")
	config.Application.Server.Mode = "test"
	config.Application.Jwt.VerifyOnlyKey = "test-verify-key"

	repoEntry := repo.NewRepoEntry()
	require.NotNil(t, repoEntry, "repo init failed")
	ctx.NewContext(context.Background(), repoEntry, nil)

	middleware.InitSession(provider)
	services.NewServices(ctx.DO())
	health.ResetForTests()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	routersv1.Router(r)

	t.Cleanup(func() {
		if sqlDB, err := repoEntry.DB().DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	return &Harness{
		Engine:   r,
		Repo:     repoEntry,
		Provider: provider,
	}
}

// SeedRoleBinding inserts a role binding for the given eiam_uid.
func (h *Harness) SeedRoleBinding(t *testing.T, eiamUid int64, role string) {
	t.Helper()
	_, err := h.Repo.RoleBinding().Upsert(models.RoleBinding{EiamUid: eiamUid, Role: role})
	require.NoError(t, err)
}

// DoRequest issues an HTTP request against the harness engine and returns the recorder.
func DoRequest(t *testing.T, engine *gin.Engine, method, path string, opts ...RequestOpt) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for _, opt := range opts {
		opt(req)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// RequestOpt modifies a test HTTP request before dispatch.
type RequestOpt func(*http.Request)

// WithAuthBearer sets the Authorization header to a bearer token.
func WithAuthBearer(token string) RequestOpt {
	return func(r *http.Request) { r.Header.Set("Authorization", "bearer "+token) }
}

// WithAPIKey sets the X-API-Key header.
func WithAPIKey(key string) RequestOpt {
	return func(r *http.Request) { r.Header.Set(middleware.ApiKeyHeader, key) }
}

// WithJSONBody sets the request body and Content-Type.
func WithJSONBody(body string) RequestOpt {
	return func(r *http.Request) {
		if body != "" {
			r.Body = io.NopCloser(strings.NewReader(body))
			r.ContentLength = int64(len(body))
		}
		r.Header.Set("Content-Type", "application/json")
	}
}

// ============ Assertion helpers ============

// AssertEnvelopeCode asserts the response body contains the expected envelope code.
func AssertEnvelopeCode(t *testing.T, w *httptest.ResponseRecorder, expectedCode int) {
	t.Helper()
	assert.Contains(t, w.Body.String(), `"code":`+strconv.Itoa(expectedCode))
}

// AssertHTTPStatus asserts the HTTP status code.
func AssertHTTPStatus(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	assert.Equal(t, expected, w.Code)
}

// AssertUnauthorized asserts 401 + envelope code 1401.
func AssertUnauthorized(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	AssertHTTPStatus(t, w, http.StatusUnauthorized)
	AssertEnvelopeCode(t, w, 1401)
}

// AssertForbidden asserts 403 + envelope code 1403.
func AssertForbidden(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	AssertHTTPStatus(t, w, http.StatusForbidden)
	AssertEnvelopeCode(t, w, 1403)
}

// AssertUnavailable asserts 503 + envelope code 1503.
func AssertUnavailable(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	AssertHTTPStatus(t, w, http.StatusServiceUnavailable)
	AssertEnvelopeCode(t, w, 1503)
}

// AssertOK asserts 200 + envelope code 0.
func AssertOK(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	AssertHTTPStatus(t, w, http.StatusOK)
	AssertEnvelopeCode(t, w, 0)
}

// AssertNotFound asserts 404 (gin default 404 has no envelope code).
func AssertNotFound(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	AssertHTTPStatus(t, w, http.StatusNotFound)
}

// ============ Health probe helpers ============

// DegradeProbeB records 3 consecutive probe B failures to flip the health state to degraded.
func DegradeProbeB() {
	health.RecordProbeB(false)
	health.RecordProbeB(false)
	health.RecordProbeB(false)
}

// RecoverProbeB records 3 consecutive probe B successes to flip back to up.
func RecoverProbeB() {
	health.RecordProbeB(true)
	health.RecordProbeB(true)
	health.RecordProbeB(true)
}

// ============ Legacy JWT generation ============

// GenerateLegacyJWT creates a signed legacy JWT token for testing the transition fallback.
func GenerateLegacyJWT(t *testing.T, uid string) string {
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

// ============ Error sentinel for session destroy ============

// DestroyedSessionErr returns redis.Nil to simulate a destroyed (logged-out) session.
func DestroyedSessionErr() error { return redis.Nil }

// ConnRefusedErr returns a connection error to simulate Redis being down.
func ConnRefusedErr() error { return errors.New("connection refused") }
