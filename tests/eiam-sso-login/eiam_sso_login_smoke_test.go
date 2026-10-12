// @feature watchalert-subproject-integration @api-functional
package eiam_sso_login

import (
	"errors"
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/internal/routers"
	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
	"github.com/stretchr/testify/assert"
)

// Smoke test: happy path for eiam-sso-login.
// Unauth access → 401, OIDC endpoints exist, valid session + role → 200,
// session destroyed → 401.
func TestEiamSsoLogin_Smoke(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-1"},
		},
	}
	h := newHarness(t, provider)
	routers.HealthCheck(h.Engine)

	// 1. Unauth access → 401
	h.Provider.Err = errors.New("no session")
	wUnauth := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, wUnauth)

	// 2. OIDC info endpoint exists
	wOidc := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/oidc/oidcInfo")
	assert.Equal(t, http.StatusOK, wOidc.Code)

	// 3. Valid session + role → 200
	h.Provider.Err = nil
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)
	wAuth := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertOK(t, wAuth)

	// 4. Session destroyed → 401
	h.Provider.Sess.GetErr = watest.DestroyedSessionErr()
	wDestroyed := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, wDestroyed)
}
