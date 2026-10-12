// @feature watchalert-subproject-integration @api-functional
package dev_subsystem_startup

import (
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"
)

// TestDevSubsystemStartupSmoke is the happy-path smoke test for the dev-subsystem-
// startup journey: server health passes, auth-health reports up, a valid session
// resolves userInfo, and the removed local login endpoint 404s.
func TestDevSubsystemStartupSmoke(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	registerHello(t, h)
	h.SeedRoleBinding(t, 1001, models.RoleUser)
	seedUser(t, h, "1001", "u1001")

	// 1. Server health.
	wHello := watest.DoRequest(t, h.Engine, http.MethodGet, "/hello")
	watest.AssertHTTPStatus(t, wHello, http.StatusOK)

	// 2. Auth-health up.
	wHealth := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/auth-health")
	watest.AssertOK(t, wHealth)

	// 3. Valid session → userInfo 200.
	wUserInfo := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/system/userInfo")
	watest.AssertHTTPStatus(t, wUserInfo, http.StatusOK)

	// 4. Local login removed → 404.
	wLogin := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/system/login",
		watest.WithJSONBody(`{"userName":"x","password":"y"}`))
	watest.AssertNotFound(t, wLogin)
}
