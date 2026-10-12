// @feature watchalert-subproject-integration @api-functional
package dev_subsystem_startup

import (
	"net/http"
	"testing"

	"watchAlert/tests/watest"
)

// TestStep4_CheckStatusAndLogs_Success verifies that both liveness (/hello) and the
// auth subsystem health (auth-health) report ready → 200.
//
// Contract: dev-subsystem-startup/step4 Outcome "success"
func TestStep4_CheckStatusAndLogs_Success(t *testing.T) {
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

// TestStep4_CheckStatusAndLogs_PartialReadyState verifies the API-testable aspect
// of a partial-ready state: the backend (liveness) is up even if other subsystems
// are not yet ready. /hello → 200.
//
// Contract: dev-subsystem-startup/step4 Outcome "partial-ready-state"
func TestStep4_CheckStatusAndLogs_PartialReadyState(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	registerHello(t, h)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/hello")
	watest.AssertHTTPStatus(t, w, http.StatusOK)
}
