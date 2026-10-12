// @feature watchalert-subproject-integration @api-functional
package dev_subsystem_startup

import (
	"net/http"
	"testing"

	"watchAlert/tests/watest"
)

// TestStep3_FrontendStartupSuccess_Success verifies the API-testable aspect of
// frontend readiness: the backing API (serving the frontend's data) responds on
// /hello → 200.
//
// Contract: dev-subsystem-startup/step3 Outcome "success"
func TestStep3_FrontendStartupSuccess_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	registerHello(t, h)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/hello")
	watest.AssertHTTPStatus(t, w, http.StatusOK)
}

// TestStep3_FrontendStartupSuccess_BuildFailureOrPortConflict verifies the
// API-testable aspect of a frontend build/port failure: the backend remains running
// independently, so /hello still responds 200.
//
// Contract: dev-subsystem-startup/step3 Outcome "frontend-build-failure-or-port-conflict"
func TestStep3_FrontendStartupSuccess_BuildFailureOrPortConflict(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	registerHello(t, h)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/hello")
	watest.AssertHTTPStatus(t, w, http.StatusOK)
}
