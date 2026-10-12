// @feature watchalert-subproject-integration @api-functional
package eiam_sso_login

import (
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
)

// Contract: eiam-sso-login/step6 Outcome "success"
// New session after relogin → 200.
func TestStep6_SessionExpiryRelogin_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-new"},
		},
	}
	h := newHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertOK(t, w)
}

// Contract: eiam-sso-login/step6 Outcome "session-expired"
// Expired session → 401.
func TestStep6_SessionExpiryRelogin_SessionExpired(t *testing.T) {
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

// Contract: eiam-sso-login/step6 Outcome "session-expired-eiam-unavailable"
// Expired session + degraded → 401 (not 503, because the read path checks
// session validity first — redis.Nil means the session is destroyed, which
// returns 401 before the degradation fallback applies).
func TestStep6_SessionExpiryRelogin_SessionExpiredEiamUnavailable(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-1"},
			GetErr:     watest.DestroyedSessionErr(),
		},
	}
	h := newHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	watest.DegradeProbeB()

	// Read with destroyed session + degraded → 401 (not 503)
	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w)
}
