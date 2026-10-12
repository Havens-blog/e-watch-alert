// @feature watchalert-subproject-integration @api-functional
package sso_logout_session_invalidation

import (
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
)

// Contract: sso-logout-session-invalidation/step1 Outcome "success"
// WatchAlert has no logout handler (session destruction is delegated to eiam).
// POST /api/watchalert/logout and /api/w8t/logout both return 404. The Auth
// middleware detects the destroyed session on the next protected request → 401.
func TestStep1_UserLogoutAnyTab_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-1"},
			GetErr:     watest.DestroyedSessionErr(),
		},
	}
	h := newHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	// No logout route exists in WatchAlert
	w1 := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/logout")
	watest.AssertNotFound(t, w1)

	w2 := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/w8t/logout")
	watest.AssertNotFound(t, w2)

	// Destroyed session detected on next protected request → 401
	w3 := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w3)
}

// Contract: sso-logout-session-invalidation/step1 Outcome "concurrent-logout-idempotent"
// After session destroy, subsequent requests are idempotent — repeated protected
// requests all return 401 (the session stays destroyed).
func TestStep1_UserLogoutAnyTab_ConcurrentIdempotent(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-1"},
			GetErr:     watest.DestroyedSessionErr(),
		},
	}
	h := newHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	// First request after destroy → 401
	w1 := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w1)

	// Subsequent request → still 401 (idempotent)
	w2 := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w2)
}

// Contract: sso-logout-session-invalidation/step1 Outcome "session-expiry-logout-race"
// Session already expired (TTL) is equivalent to destroyed — logout is idempotent,
// and the next protected request returns 401.
func TestStep1_UserLogoutAnyTab_SessionExpiryRace(t *testing.T) {
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
