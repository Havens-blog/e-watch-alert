// @feature watchalert-subproject-integration @api-functional
package eiam_sso_login

import (
	"errors"
	"net/http"
	"testing"

	"watchAlert/internal/routers"
	"watchAlert/tests/watest"

	"github.com/stretchr/testify/assert"
)

// Contract: eiam-sso-login/step1 Outcome "success"
// Unauthenticated access: the platform is reachable (GET /hello → 200), but
// auth-protected endpoints return 401.
func TestStep1_AccessUnauthPlatform_Success(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no session")}
	h := newHarness(t, provider)
	routers.HealthCheck(h.Engine) // register /hello route

	// Platform reachable
	wHello := watest.DoRequest(t, h.Engine, http.MethodGet, "/hello")
	assert.Equal(t, http.StatusOK, wHello.Code)

	// Auth-protected endpoint → 401
	wProtected := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/system/userInfo")
	watest.AssertUnauthorized(t, wProtected)
}

// Contract: eiam-sso-login/step1 Outcome "local-login-endpoint-404"
// POST /api/system/login has been removed → 404.
func TestStep1_AccessUnauthPlatform_LocalLogin404(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no session")}
	h := newHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/system/login",
		watest.WithJSONBody(`{"username":"admin","password":"admin"}`))
	watest.AssertNotFound(t, w)
}
