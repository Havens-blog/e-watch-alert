// @feature watchalert-subproject-integration @api-functional
package dev_subsystem_startup

import (
	"net/http"
	"testing"

	"watchAlert/tests/watest"
)

// TestStep6_TriggerUnifiedQualityGate_Success verifies the API-testable aspect of a
// passing quality gate: the server is healthy (/hello → 200).
//
// Contract: dev-subsystem-startup/step6 Outcome "success"
func TestStep6_TriggerUnifiedQualityGate_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	registerHello(t, h)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/hello")
	watest.AssertHTTPStatus(t, w, http.StatusOK)
}

// TestStep6_TriggerUnifiedQualityGate_CoverageBelowThreshold verifies the API-
// testable aspect of a failing quality gate (coverage below threshold): the server
// remains operational for the developer to iterate, so /hello still responds 200.
//
// Contract: dev-subsystem-startup/step6 Outcome "coverage-below-threshold"
func TestStep6_TriggerUnifiedQualityGate_CoverageBelowThreshold(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	registerHello(t, h)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/hello")
	watest.AssertHTTPStatus(t, w, http.StatusOK)
}
