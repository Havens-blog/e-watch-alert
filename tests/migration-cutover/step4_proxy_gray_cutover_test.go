// @feature watchalert-subproject-integration @api-functional
package migration_cutover

import (
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"
)

// TestStep4_ProxyGrayCutover_Success verifies that after gray cutover the new
// instance serves traffic: /hello → 200 and auth-health → 200.
//
// Contract: migration-cutover/step4 Outcome "success"
func TestStep4_ProxyGrayCutover_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	registerHello(t, h)

	wHello := watest.DoRequest(t, h.Engine, http.MethodGet, "/hello")
	watest.AssertHTTPStatus(t, wHello, http.StatusOK)

	wHealth := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/auth-health")
	watest.AssertOK(t, wHealth)
}

// TestStep4_ProxyGrayCutover_MetricsBelowThresholdRollback verifies that if cutover
// metrics dip below threshold (auth degradation), writes are blocked (503) — the
// rollback signal.
//
// Contract: migration-cutover/step4 Outcome "cutover-metrics-below-threshold-rollback"
func TestStep4_ProxyGrayCutover_MetricsBelowThresholdRollback(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	watest.DegradeProbeB()

	w := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/role-bindings",
		watest.WithJSONBody(`{"bindings":[{"eiam_uid":4001,"role":"user"}]}`))
	watest.AssertUnavailable(t, w)
}

// TestStep4_ProxyGrayCutover_EiamUnavailableDuringCutover verifies the mixed-mode
// behavior when eiam is unavailable during cutover: writes → 503, reads with a valid
// session → 200 (grace read path).
//
// Contract: migration-cutover/step4 Outcome "eiam-unavailable-during-cutover"
func TestStep4_ProxyGrayCutover_EiamUnavailableDuringCutover(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	watest.DegradeProbeB()

	// Write → 503.
	wWrite := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/role-bindings",
		watest.WithJSONBody(`{"bindings":[{"eiam_uid":4002,"role":"user"}]}`))
	watest.AssertUnavailable(t, wWrite)

	// Read with valid session → 200 (degraded read path tolerates Redis-healthy session).
	wRead := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertOK(t, wRead)
}
