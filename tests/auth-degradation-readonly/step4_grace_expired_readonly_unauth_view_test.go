// @feature watchalert-subproject-integration @api-functional
package auth_degradation_readonly

import (
	"errors"
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
	"github.com/stretchr/testify/assert"
)

// TestStep4_GraceExpiredReadonlyUnauthView_Success verifies that once the grace
// window has effectively expired (degraded + no session), unauthenticated users are
// rejected from protected endpoints (401) while the public auth-health probe still
// reports the degraded status.
//
// Contract: auth-degradation-readonly/step4 Outcome "success"
func TestStep4_GraceExpiredReadonlyUnauthView_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step4-success"}},
		Err:  errors.New("no session"),
	}
	h := watest.NewHarness(t, provider)

	watest.DegradeProbeB()

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w)

	wh := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/auth-health")
	watest.AssertOK(t, wh)
	assert.Contains(t, wh.Body.String(), `"probe_b":"degraded"`)
	assert.Contains(t, wh.Body.String(), `"write_disabled":true`)
}

// TestStep4_GraceExpiryBoundaryRace verifies that a write attempted during grace
// expiry is rejected with 503 regardless of timing — writes are never allowed once
// degraded, even at the boundary.
//
// Contract: auth-degradation-readonly/step4 Outcome "grace-expiry-boundary-race"
func TestStep4_GraceExpiryBoundaryRace(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step4-race"}},
	}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	watest.DegradeProbeB()

	w := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/role-bindings",
		watest.WithJSONBody(`{"bindings":[{"eiam_uid":2003,"role":"user"}]}`))
	watest.AssertUnavailable(t, w)
}

// TestStep4_ReadonlyViewSensitiveFieldMasking verifies that an unauthenticated read
// returns 401 with data:null — no sensitive payload is leaked in the rejection
// envelope.
//
// Contract: auth-degradation-readonly/step4 Outcome "readonly-view-sensitive-field-masking"
func TestStep4_ReadonlyViewSensitiveFieldMasking(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step4-masking"}},
		Err:  errors.New("no session"),
	}
	h := watest.NewHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w)
	// The 401 envelope must carry data:null (no sensitive field leakage).
	assert.Contains(t, w.Body.String(), `"data":null`)
}
