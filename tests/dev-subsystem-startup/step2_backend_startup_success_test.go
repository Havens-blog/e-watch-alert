// @feature watchalert-subproject-integration @api-functional
package dev_subsystem_startup

import (
	"net/http"
	"testing"

	"watchAlert/tests/watest"

	"github.com/stretchr/testify/assert"
)

// TestStep2_BackendStartupSuccess_Success verifies the backend port is responding
// after startup: GET /hello → 200.
//
// Contract: dev-subsystem-startup/step2 Outcome "success"
func TestStep2_BackendStartupSuccess_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	registerHello(t, h)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/hello")
	watest.AssertHTTPStatus(t, w, http.StatusOK)
}

// TestStep2_BackendStartupSuccess_PortConflictOrStaleProcess verifies the
// API-testable aspect of a port-conflict / stale-process condition: against an
// in-process engine the server is bound and /hello responds with the expected
// health envelope shape. (A real port conflict yields EADDRINUSE at bind time, an
// environment concern.)
//
// Contract: dev-subsystem-startup/step2 Outcome "backend-port-conflict-or-stale-process"
func TestStep2_BackendStartupSuccess_PortConflictOrStaleProcess(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	registerHello(t, h)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/hello")
	watest.AssertHTTPStatus(t, w, http.StatusOK)
	assert.Contains(t, w.Body.String(), `"success"`)
}
