// @feature watchalert-subproject-integration @api-functional
package local_login_removal_verification

import (
	"errors"
	"net/http"
	"testing"

	"watchAlert/tests/watest"
)

// Smoke test: happy path for local-login-removal-verification.
// Local login is removed (404), legacy JWT read is tolerated (200),
// and auth-health responds.
func TestLocalLoginRemoval_Smoke(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no shared session")}
	h := newHarness(t, provider)

	// 1. Local login endpoint removed → 404
	wLogin := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/system/login",
		watest.WithJSONBody(`{"username":"admin","password":"admin"}`))
	watest.AssertNotFound(t, wLogin)

	// 2. Legacy JWT on read endpoint → tolerated (200)
	token := watest.GenerateLegacyJWT(t, "uid-legacy")
	wRead := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings",
		func(r *http.Request) { r.Header.Set("Authorization", token) })
	watest.AssertOK(t, wRead)

	// 3. Auth health works (public endpoint)
	wHealth := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/auth-health")
	watest.AssertOK(t, wHealth)
}
