// @feature watchalert-subproject-integration @api-functional
package sso_logout_session_invalidation

import (
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
)

// Smoke test: happy path for sso-logout-session-invalidation.
// Valid session → 200; after session destroy → 401 on next request.
func TestSsoLogout_Smoke(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-1"},
		},
	}
	h := newHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	// 1. Valid session → 200
	w1 := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertOK(t, w1)

	// 2. Destroy session → 401 on next request
	provider.Sess.GetErr = watest.DestroyedSessionErr()
	w2 := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w2)
}
