// @feature watchalert-subproject-integration @api-functional
package sso_logout_session_invalidation

import (
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
)

// Contract: sso-logout-session-invalidation/step3 Outcome "success"
// After 401, the frontend redirects to eiam login. API aspect: verify the 401
// response is returned (the frontend interceptor handles the redirect, not the API).
func TestStep3_OtherTabConvergeRedirect_Success(t *testing.T) {
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

// Contract: sso-logout-session-invalidation/step3 Outcome "session-expired"
// Session TTL expiry → 401 on next request.
func TestStep3_OtherTabConvergeRedirect_SessionExpired(t *testing.T) {
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

// Contract: sso-logout-session-invalidation/step3 Outcome "session-expired-mid-edit-unsaved"
// A write request with an expired/destroyed session → 401 (when not in degraded
// state, session invalidity takes precedence over the write path).
func TestStep3_OtherTabConvergeRedirect_SessionExpiredMidEdit(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-1"},
			GetErr:     watest.DestroyedSessionErr(),
		},
	}
	h := newHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	w := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/role-bindings",
		watest.WithJSONBody(`{"bindings":[]}`))
	watest.AssertUnauthorized(t, w)
}
