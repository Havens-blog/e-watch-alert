// @feature watchalert-subproject-integration @api-functional
package local_login_removal_verification

import (
	"errors"
	"net/http"
	"testing"

	"watchAlert/tests/watest"

	"github.com/stretchr/testify/assert"
)

// Contract: local-login-removal-verification/step1 Outcome "success"
// POST /api/system/login has been removed → 404, no token issued.
func TestStep1_LocalLoginRemoved_Success(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no session")}
	h := newHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/system/login",
		watest.WithJSONBody(`{"username":"admin","password":"admin"}`))

	watest.AssertNotFound(t, w)
	assert.NotContains(t, w.Body.String(), "token",
		"removed endpoint must not issue a token")
}

// Contract: local-login-removal-verification/step1 Outcome "endpoint-returns-401-not-404"
// Security regression guard: the removed endpoint must return 404, not 401.
// A 401 would imply the route still exists behind auth middleware.
func TestStep1_LocalLoginRemoved_Returns404Not401(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no session")}
	h := newHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/system/login",
		watest.WithJSONBody(`{"username":"admin","password":"admin"}`))

	assert.Equal(t, http.StatusNotFound, w.Code,
		"removed login endpoint must return 404, not 401")
	assert.NotContains(t, w.Body.String(), `"code":1401`,
		"response must not carry the 401 envelope code")
}

// Contract: local-login-removal-verification/step1 Outcome "endpoint-returns-200-issues-token"
// Critical backdoor guard: the removed endpoint must NOT return 200 or issue a token.
func TestStep1_LocalLoginRemoved_DoesNotReturn200(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no session")}
	h := newHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/system/login",
		watest.WithJSONBody(`{"username":"admin","password":"admin"}`))

	assert.NotEqual(t, http.StatusOK, w.Code,
		"removed endpoint must never return 200 (would be a backdoor)")
	assert.NotContains(t, w.Body.String(), "token",
		"no token must be present in the response")
}

// Contract: local-login-removal-verification/step1 Outcome "unauthorized"
// Requesting a protected endpoint without a valid session → 401, no sensitive data.
func TestStep1_LocalLoginRemoved_Unauthorized(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no session")}
	h := newHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/system/userInfo")

	watest.AssertUnauthorized(t, w)
	assert.Contains(t, w.Body.String(), `"data":null`,
		"401 response must not leak sensitive data")
}
