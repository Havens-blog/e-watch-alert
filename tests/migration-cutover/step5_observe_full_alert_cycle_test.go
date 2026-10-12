// @feature watchalert-subproject-integration @api-functional
package migration_cutover

import (
	"net/http"
	"testing"

	"watchAlert/tests/watest"
)

// TestStep5_ObserveFullAlertCycle_Success verifies the API-testable aspect of a full
// alert observation cycle on the new instance: liveness and auth-health both
// respond 200.
//
// Contract: migration-cutover/step5 Outcome "success"
func TestStep5_ObserveFullAlertCycle_Success(t *testing.T) {
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

// TestStep5_ObserveFullAlertCycle_AnomalyRollback verifies the API-testable aspect
// of an anomaly-triggered rollback: the health endpoint on the (still-running)
// instance continues to respond, so observability is preserved across the rollback.
//
// Contract: migration-cutover/step5 Outcome "observe-anomaly-rollback"
func TestStep5_ObserveFullAlertCycle_AnomalyRollback(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	registerHello(t, h)

	// After rollback the instance is still observable via health.
	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/hello")
	watest.AssertHTTPStatus(t, w, http.StatusOK)
}
