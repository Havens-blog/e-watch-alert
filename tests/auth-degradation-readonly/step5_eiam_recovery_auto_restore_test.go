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

// TestStep5_EiamRecoveryAutoRestore_Success verifies that 3 consecutive probe B
// successes flip the state back to up, re-enabling writes (write_disabled=false).
//
// Contract: auth-degradation-readonly/step5 Outcome "success"
func TestStep5_EiamRecoveryAutoRestore_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step5-success"}},
	}
	h := watest.NewHarness(t, provider)

	watest.DegradeProbeB() // → degraded
	watest.RecoverProbeB() // 3 successes → up

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/auth-health")
	watest.AssertOK(t, w)
	assert.Contains(t, w.Body.String(), `"probe_b":"up"`)
	assert.Contains(t, w.Body.String(), `"write_disabled":false`)
}

// TestStep5_AvailabilityFlutterHysteresis verifies that a single success after
// degradation is NOT enough to recover — hysteresis requires 3 consecutive
// successes, so flapping availability stays degraded.
//
// Contract: auth-degradation-readonly/step5 Outcome "availability-flutter-hysteresis"
func TestStep5_AvailabilityFlutterHysteresis(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step5-flutter"}},
	}
	h := watest.NewHarness(t, provider)

	watest.DegradeProbeB()    // → degraded
	health.RecordProbeB(true) // single success → sameCount=1, still degraded

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/auth-health")
	watest.AssertOK(t, w)
	assert.Contains(t, w.Body.String(), `"probe_b":"degraded"`)
	assert.Contains(t, w.Body.String(), `"write_disabled":true`)
}

// TestStep5_AuthHealthPollingDuringDegradation verifies the public auth-health
// endpoint reflects the live state during degradation and after recovery, so a
// polling frontend can drive the degraded UI and detect restoration.
//
// Contract: auth-degradation-readonly/step5 Outcome "auth-health-polling-during-degradation"
func TestStep5_AuthHealthPollingDuringDegradation(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step5-poll"}},
	}
	h := watest.NewHarness(t, provider)

	// During degradation → polling sees degraded.
	watest.DegradeProbeB()
	wDegraded := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/auth-health")
	watest.AssertOK(t, wDegraded)
	assert.Contains(t, wDegraded.Body.String(), `"probe_b":"degraded"`)
	assert.Contains(t, wDegraded.Body.String(), `"write_disabled":true`)

	// After recovery → polling sees up.
	watest.RecoverProbeB()
	wRecovered := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/auth-health")
	watest.AssertOK(t, wRecovered)
	assert.Contains(t, wRecovered.Body.String(), `"probe_b":"up"`)
	assert.Contains(t, wRecovered.Body.String(), `"write_disabled":false`)
}
