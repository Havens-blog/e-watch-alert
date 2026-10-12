// @feature watchalert-subproject-integration @api-functional
package auth_degradation_readonly

import (
	"net/http"
	"testing"

	"watchAlert/internal/health"
	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
	"github.com/stretchr/testify/assert"
)

// TestStep1_DegradationTrigger_Success verifies that 3 consecutive probe B
// failures flip the auth subsystem into degraded mode, exposing the state via
// the public auth-health endpoint.
//
// Contract: auth-degradation-readonly/step1 Outcome "success"
func TestStep1_DegradationTrigger_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step1-success"}},
	}
	h := watest.NewHarness(t, provider)

	// 3 consecutive probe B failures → hysteresis threshold (3) reached → degraded.
	watest.DegradeProbeB()

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/auth-health")
	watest.AssertOK(t, w)
	assert.Contains(t, w.Body.String(), `"probe_b":"degraded"`)
	assert.Contains(t, w.Body.String(), `"write_disabled":true`)
}

// TestStep1_DegradationTrigger_ProbeBelowThreshold verifies that 1-2 probe B
// failures do NOT flip the state (hysteresis requires 3 consecutive same-direction
// observations). auth-health must still report probe_b=up, write_disabled=false.
//
// Contract: auth-degradation-readonly/step1 Outcome "probe-below-threshold"
func TestStep1_DegradationTrigger_ProbeBelowThreshold(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step1-threshold"}},
	}
	h := watest.NewHarness(t, provider)

	// NewHarness calls health.ResetForTests(), so probe B starts fresh (up, sameCount=0).
	// Only 2 failures → below the 3-observation hysteresis threshold → state stays up.
	health.RecordProbeB(false)
	health.RecordProbeB(false)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/auth-health")
	watest.AssertOK(t, w)
	assert.Contains(t, w.Body.String(), `"probe_b":"up"`)
	assert.Contains(t, w.Body.String(), `"write_disabled":false`)
}
