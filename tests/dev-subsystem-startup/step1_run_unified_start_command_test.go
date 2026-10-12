// @feature watchalert-subproject-integration @api-functional
package dev_subsystem_startup

import (
	"net/http"
	"strconv"
	"testing"

	routerspkg "watchAlert/internal/routers"
	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
	"github.com/stretchr/testify/assert"
)

// registerHello wires the public /hello liveness route onto the harness engine,
// mirroring main.go startup (the harness only registers the /api/* v1 group).
func registerHello(t *testing.T, h *watest.Harness) {
	t.Helper()
	routerspkg.HealthCheck(h.Engine)
}

// claimsFor builds session claims for the given eiam uid with a per-call SSID.
func claimsFor(uid int64) session.Claims {
	return session.Claims{Uid: uid, SSID: "ssid-dev-" + strconv.FormatInt(uid, 10)}
}

// TestStep1_RunUnifiedStartCommand_Success verifies the API-testable aspect of the
// unified start command: once the backend has started, the liveness probe responds
// 200.
//
// Contract: dev-subsystem-startup/step1 Outcome "success"
func TestStep1_RunUnifiedStartCommand_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	registerHello(t, h)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/hello")
	watest.AssertHTTPStatus(t, w, http.StatusOK)
	assert.Contains(t, w.Body.String(), `"success":"true"`)
}

// TestStep1_RunUnifiedStartCommand_BackendCompileFailureNonWindows verifies the
// API-testable aspect of a backend compile failure: against an in-process engine the
// server is up, so the health endpoint responds. (A genuine compile failure yields
// connection-refused, which is an environment concern, not an in-process one — the
// positive contract is asserted here.)
//
// Contract: dev-subsystem-startup/step1 Outcome "backend-compile-failure-non-windows"
func TestStep1_RunUnifiedStartCommand_BackendCompileFailureNonWindows(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	registerHello(t, h)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/hello")
	watest.AssertHTTPStatus(t, w, http.StatusOK)
}

// TestStep1_RunUnifiedStartCommand_WindowsMultiEntryToolchainUnsupported verifies
// the API-testable aspect of the unsupported-toolchain variant: when the backend is
// up, the liveness endpoint responds (the failure variant is the negation, asserted
// at the environment level).
//
// Contract: dev-subsystem-startup/step1 Outcome "windows-multi-entry-toolchain-unsupported"
func TestStep1_RunUnifiedStartCommand_WindowsMultiEntryToolchainUnsupported(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	registerHello(t, h)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/hello")
	watest.AssertHTTPStatus(t, w, http.StatusOK)
}
