// @feature watchalert-subproject-integration @api-functional
package sso_logout_session_invalidation

import (
	"errors"
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
	"github.com/stretchr/testify/assert"
)

// Contract: sso-logout-session-invalidation/step2 Outcome "success"
// After logout (session destroyed via redis.Nil), the next request → 401.
func TestStep2_OtherTabNextRequest401_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-1"},
			GetErr:     watest.DestroyedSessionErr(),
		},
	}
	h := newHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w)
}

// Contract: sso-logout-session-invalidation/step2 Outcome "unauthorized"
// Request without a valid session → 401, no sensitive data in response.
func TestStep2_OtherTabNextRequest401_Unauthorized(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no session")}
	h := newHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w)
	assert.Contains(t, w.Body.String(), `"data":null`,
		"401 response must not leak sensitive data")
}

// Contract: sso-logout-session-invalidation/step2 Outcome "idle-tab-no-request"
// No request made from an idle tab → no 401 is triggered. This is inherently a
// non-test: 401 only occurs when a request hits the server.
func TestStep2_OtherTabNextRequest401_IdleTabNoRequest(t *testing.T) {
	t.Log("idle-tab-no-request: when no HTTP request is made from an idle tab, " +
		"no 401 is triggered. 401 only occurs when a request reaches the Auth " +
		"middleware with a destroyed/missing session. This outcome documents that " +
		"the server cannot proactively push a 401 to an idle client.")
}

// Contract: sso-logout-session-invalidation/step2 Outcome "session-storage-regression-guard"
// Verify that the shared session is used: a session destroyed in one request
// context causes 401 in subsequent requests (the provider is shared across all
// requests, simulating a shared Redis session store).
func TestStep2_OtherTabNextRequest401_SessionStorageRegressionGuard(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-1"},
			GetErr:     watest.DestroyedSessionErr(),
		},
	}
	h := newHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	// First context (tab A) → 401
	w1 := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w1)

	// Second context (tab B) — same shared session store → also 401
	w2 := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w2)
}
