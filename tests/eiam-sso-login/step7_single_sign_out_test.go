// @feature watchalert-subproject-integration @api-functional
package eiam_sso_login

import (
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
)

// Contract: eiam-sso-login/step7 Outcome "success"
// Session destroyed (by eiam) → next request 401. WatchAlert has no logout
// handler; eiam destroys the shared session, and the Auth middleware detects
// the destroyed session on the next protected request.
func TestStep7_SingleSignOut_Success(t *testing.T) {
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

// Contract: eiam-sso-login/step7 Outcome "other-tab-401"
// Another tab's request after logout → 401 (shared session destroyed).
func TestStep7_SingleSignOut_OtherTab401(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-1"},
			GetErr:     watest.DestroyedSessionErr(),
		},
	}
	h := newHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	// First request (tab A) → 401
	w1 := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w1)

	// Second request (tab B) → also 401
	w2 := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w2)
}
