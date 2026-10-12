// @feature watchalert-subproject-integration @api-functional
package eiam_sso_login

import (
	"errors"
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
)

// Contract: eiam-sso-login/step4 Outcome "success"
// Valid session + admin role → 200.
func TestStep4_ValidateSessionMatchRole_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-1"},
		},
	}
	h := newHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertOK(t, w)
}

// Contract: eiam-sso-login/step4 Outcome "unauthorized"
// No session → 401.
func TestStep4_ValidateSessionMatchRole_Unauthorized(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no session")}
	h := newHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w)
}

// Contract: eiam-sso-login/step4 Outcome "no-role"
// Valid session + no role binding → 403.
func TestStep4_ValidateSessionMatchRole_NoRole(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 9999, SSID: "ssid-norole"},
		},
	}
	h := newHarness(t, provider)
	// Deliberately no SeedRoleBinding for uid 9999

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertForbidden(t, w)
}

// Contract: eiam-sso-login/step4 Outcome "invalid-role-binding"
// Valid session + invalid role binding (binding exists for a different uid,
// so the session's uid has no valid binding) → 403 (same as no-role).
func TestStep4_ValidateSessionMatchRole_InvalidRoleBinding(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 9999, SSID: "ssid-invalid"},
		},
	}
	h := newHarness(t, provider)
	// Seed a binding for uid 1001, but session is for uid 9999 → no match → 403
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertForbidden(t, w)
}

// Contract: eiam-sso-login/step4 Outcome "legacy-jwt-read-only"
// Legacy JWT on read → 200; on write → 403.
func TestStep4_ValidateSessionMatchRole_LegacyJWTReadOnly(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no shared session")}
	h := newHarness(t, provider)

	token := watest.GenerateLegacyJWT(t, "uid-legacy")

	// Read → 200
	wRead := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings",
		func(r *http.Request) { r.Header.Set("Authorization", token) })
	watest.AssertOK(t, wRead)

	// Write → 403
	wWrite := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/role-bindings",
		watest.WithJSONBody(`{"bindings":[]}`),
		func(r *http.Request) { r.Header.Set("Authorization", token) })
	watest.AssertForbidden(t, wWrite)
}
