// @feature watchalert-subproject-integration @api-functional
package eiam_sso_login

import (
	"errors"
	"net/http"
	"testing"

	"watchAlert/internal/health"
	"watchAlert/internal/models"
	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
	"github.com/stretchr/testify/assert"
)

// Contract: eiam-sso-login/step2 Outcome "success"
// The OIDC info endpoint exists (GET /api/oidc/oidcInfo → 200), confirming the
// redirect-to-eiam flow has a configured endpoint.
func TestStep2_RedirectToEiamLogin_Success(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no session")}
	h := newHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/oidc/oidcInfo")
	assert.Equal(t, http.StatusOK, w.Code)
}

// Contract: eiam-sso-login/step2 Outcome "eiam-unavailable-readonly"
// eiam degraded → auth-health shows degraded, write → 503, read with valid
// session → 200 (cache fallback / session still valid).
func TestStep2_RedirectToEiamLogin_EiamUnavailableReadonly(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-1"},
		},
	}
	h := newHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	// Flip probe B to degraded (3 consecutive failures)
	watest.DegradeProbeB()

	// auth-health shows degraded
	wHealth := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/auth-health")
	assert.Contains(t, wHealth.Body.String(), `"probe_b":"degraded"`)
	assert.Contains(t, wHealth.Body.String(), `"write_disabled":true`)

	// Write → 503 (degraded early-return before session read)
	wWrite := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/role-bindings",
		watest.WithJSONBody(`{"bindings":[]}`))
	watest.AssertUnavailable(t, wWrite)

	// Read with valid session → 200
	wRead := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertOK(t, wRead)
}

// Contract: eiam-sso-login/step2 Outcome "probe-below-threshold"
// 1-2 probe B failures → not degraded (hysteresis threshold is 3; auth-health
// still shows up).
func TestStep2_RedirectToEiamLogin_ProbeBelowThreshold(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no session")}
	h := newHarness(t, provider)

	// 2 failures — below the 3-failure hysteresis threshold
	health.RecordProbeB(false)
	health.RecordProbeB(false)

	wHealth := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/auth-health")
	assert.Contains(t, wHealth.Body.String(), `"probe_b":"up"`)
	assert.Contains(t, wHealth.Body.String(), `"write_disabled":false`)
}

// Contract: eiam-sso-login/step2 Outcome "degradation-grace-window"
// Valid session + degraded → read 200, write 503 (grace window for reads).
func TestStep2_RedirectToEiamLogin_DegradationGraceWindow(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-1"},
		},
	}
	h := newHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	watest.DegradeProbeB()

	// Read → 200 (grace window: valid session, read path not gated by WriteDisabled)
	wRead := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertOK(t, wRead)

	// Write → 503 (write path gated by WriteDisabled)
	wWrite := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/role-bindings",
		watest.WithJSONBody(`{"bindings":[]}`))
	watest.AssertUnavailable(t, wWrite)
}
