// @feature watchalert-subproject-integration @api-functional
package eiam_sso_login

import (
	"errors"
	"net/http"
	"testing"

	"watchAlert/tests/watest"

	"github.com/stretchr/testify/assert"
)

// Contract: eiam-sso-login/step3 Outcome "success"
// The OIDC callback endpoint exists (GET /api/oidc/callback without proper
// params returns an error, but not 404 — the endpoint is registered).
func TestStep3_CompleteEiamAuth_Success(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no session")}
	h := newHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/oidc/callback")
	assert.NotEqual(t, http.StatusNotFound, w.Code,
		"callback endpoint must exist (not 404)")
}

// Contract: eiam-sso-login/step3 Outcome "auth-failure"
// Auth failure → no session created → protected endpoint returns 401.
func TestStep3_CompleteEiamAuth_AuthFailure(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("auth failure")}
	h := newHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w)
}

// Contract: eiam-sso-login/step3 Outcome "redirect-whitelist-fail"
// OIDC callback with a malicious redirect param → handled (endpoint exists,
// does not crash). The actual whitelist enforcement lives in eiam.
func TestStep3_CompleteEiamAuth_RedirectWhitelistFail(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no session")}
	h := newHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodGet,
		"/api/oidc/callback?redirect=https://evil.com")
	assert.NotEqual(t, http.StatusNotFound, w.Code,
		"callback endpoint must handle malicious redirect params (not 404)")
}
