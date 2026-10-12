// @feature watchalert-subproject-integration @api-functional
package dev_subsystem_startup

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"

	"github.com/stretchr/testify/require"
)

// seedUser inserts a Member row whose UserId matches the eiam_uid string the Auth
// middleware injects for a non-admin role binding, so GET /api/system/userInfo can
// resolve a real user.
func seedUser(t *testing.T, h *watest.Harness, userId, userName string) {
	t.Helper()
	require.NoError(t, h.Repo.User().Create(models.Member{
		UserId:   userId,
		UserName: userName,
		CreateAt: time.Now().Unix(),
	}))
}

// TestStep5_AccessPlatformStartWork_Success verifies the platform is reachable and
// the eiam-backed login flow works: GET /api/oidc/oidcInfo → 200, and a valid shared
// session resolves GET /api/system/userInfo → 200.
//
// Contract: dev-subsystem-startup/step5 Outcome "success"
func TestStep5_AccessPlatformStartWork_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	// RoleUser → Auth injects UserId = strconv(uid) = "1001".
	h.SeedRoleBinding(t, 1001, models.RoleUser)
	seedUser(t, h, "1001", "u1001")

	wOidc := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/oidc/oidcInfo")
	watest.AssertHTTPStatus(t, wOidc, http.StatusOK)

	wUserInfo := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/system/userInfo")
	watest.AssertHTTPStatus(t, wUserInfo, http.StatusOK)
}

// TestStep5_AccessPlatformStartWork_SessionExpired verifies that a destroyed
// session (redis.Nil) is rejected with 401, code 1401.
//
// Contract: dev-subsystem-startup/step5 Outcome "session-expired"
func TestStep5_AccessPlatformStartWork_SessionExpired(t *testing.T) {
	sess := &watest.FakeSession{ClaimsData: claimsFor(1001)}
	sess.GetErr = watest.DestroyedSessionErr()
	provider := &watest.FakeProvider{Sess: sess}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleUser)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/system/userInfo")
	watest.AssertUnauthorized(t, w)
}

// TestStep5_AccessPlatformStartWork_Unauthorized401 verifies that a missing session
// is rejected with 401, code 1401.
//
// Contract: dev-subsystem-startup/step5 Outcome "unauthorized-401"
func TestStep5_AccessPlatformStartWork_Unauthorized401(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
		Err:  errors.New("no session"),
	}
	h := watest.NewHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/system/userInfo")
	watest.AssertUnauthorized(t, w)
}

// TestStep5_AccessPlatformStartWork_Forbidden403 verifies that a valid session
// without a role binding is denied with 403, code 1403.
//
// Contract: dev-subsystem-startup/step5 Outcome "forbidden-403"
func TestStep5_AccessPlatformStartWork_Forbidden403(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	// Intentionally NO role binding seeded.

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/system/userInfo")
	watest.AssertForbidden(t, w)
}

// TestStep5_AccessPlatformStartWork_ValidationError verifies that an invalid form
// (empty bindings array on a write endpoint) is rejected with 400, code 1400.
//
// Contract: dev-subsystem-startup/step5 Outcome "validation-error"
func TestStep5_AccessPlatformStartWork_ValidationError(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	w := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/role-bindings",
		watest.WithJSONBody(`{"bindings":[]}`))
	watest.AssertHTTPStatus(t, w, http.StatusBadRequest)
	watest.AssertEnvelopeCode(t, w, 1400)
}

// TestStep5_AccessPlatformStartWork_Unavailable503 verifies that during auth
// degradation a write is rejected with 503, code 1503.
//
// Contract: dev-subsystem-startup/step5 Outcome "unavailable-503"
func TestStep5_AccessPlatformStartWork_Unavailable503(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	watest.DegradeProbeB()

	w := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/role-bindings",
		watest.WithJSONBody(`{"bindings":[{"eiam_uid":5001,"role":"user"}]}`))
	watest.AssertUnavailable(t, w)
}

// TestStep5_AccessPlatformStartWork_LocalLoginRemoved404 verifies that the removed
// local login endpoint (POST /api/system/login) is no longer registered → 404.
//
// Contract: dev-subsystem-startup/step5 Outcome "local-login-removed-404"
func TestStep5_AccessPlatformStartWork_LocalLoginRemoved404(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)

	w := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/system/login",
		watest.WithJSONBody(`{"userName":"x","password":"y"}`))
	watest.AssertNotFound(t, w)
}
