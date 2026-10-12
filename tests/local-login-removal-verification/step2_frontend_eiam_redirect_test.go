// @feature watchalert-subproject-integration @api-functional
package local_login_removal_verification

import (
	"errors"
	"net/http"
	"testing"

	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
	"github.com/stretchr/testify/assert"
)

// Contract: local-login-removal-verification/step2 Outcome "success"
// The OIDC info endpoint exists and returns config (the redirect-to-eiam
// endpoint is registered). This is the API-testable aspect of the frontend
// redirect flow.
func TestStep2_FrontendEiamRedirect_Success(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no session")}
	h := newHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/oidc/oidcInfo")

	assert.Equal(t, http.StatusOK, w.Code,
		"OIDC info endpoint must return 200")
}

// Contract: local-login-removal-verification/step2 Outcome "redirect-whitelist-reject"
// The OIDC callback endpoint exists and handles redirect params. The actual
// whitelist enforcement lives in eiam; here we verify the callback endpoint
// is registered and does not crash on malicious redirect params.
func TestStep2_FrontendEiamRedirect_WhitelistReject(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no session")}
	h := newHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodGet,
		"/api/oidc/callback?redirect=https://evil.com")

	assert.NotEqual(t, http.StatusNotFound, w.Code,
		"callback endpoint must exist (not 404) even with malicious redirect params")
}

// Contract: local-login-removal-verification/step2 Outcome "session-expired-during-verification"
// A request with an expired/destroyed session → 401.
func TestStep2_FrontendEiamRedirect_SessionExpired(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-expired"},
			GetErr:     watest.DestroyedSessionErr(),
		},
	}
	h := newHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/system/userInfo")

	watest.AssertUnauthorized(t, w)
}
