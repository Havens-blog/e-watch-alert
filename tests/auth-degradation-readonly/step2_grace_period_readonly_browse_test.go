// @feature watchalert-subproject-integration @api-functional
package auth_degradation_readonly

import (
	"errors"
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
	"github.com/stretchr/testify/assert"
)

// TestStep2_GracePeriodReadonlyBrowse_Success verifies that in degraded mode a
// read endpoint still serves a valid shared session (grace-period read-only browse).
//
// Contract: auth-degradation-readonly/step2 Outcome "success"
func TestStep2_GracePeriodReadonlyBrowse_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step2-success"}},
	}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	watest.DegradeProbeB()

	// GET (read) with a valid session → Auth passes degraded read path → 200.
	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertOK(t, w)
}

// TestStep2_GracePeriodRedisSessionIndepFailure verifies that degradation does NOT
// extend the lifetime of a destroyed session: redis.Nil (logout) → 401 even while
// degraded. The two failure axes are independent.
//
// Contract: auth-degradation-readonly/step2 Outcome "grace-period-redis-session-indep-failure"
func TestStep2_GracePeriodRedisSessionIndepFailure(t *testing.T) {
	sess := &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step2-indepfail"}}
	provider := &watest.FakeProvider{Sess: sess}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	watest.DegradeProbeB()
	// Session was destroyed (redis.Nil) → 401 regardless of degradation.
	sess.GetErr = watest.DestroyedSessionErr()

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w)
}

// TestStep2_SessionTTLExpiryNonDegradation verifies that a destroyed session yields
// 401 in the normal (non-degraded) state — degradation is not required to reject
// expired sessions.
//
// Contract: auth-degradation-readonly/step2 Outcome "session-ttl-expiry-non-degradation"
func TestStep2_SessionTTLExpiryNonDegradation(t *testing.T) {
	sess := &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step2-ttl"}}
	sess.GetErr = watest.DestroyedSessionErr()
	provider := &watest.FakeProvider{Sess: sess}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	// Fresh (non-degraded) state + destroyed session → 401.
	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w)
}

// TestStep2_UnloggedUserReadonlyEntry verifies that an unauthenticated user during
// degradation is rejected from protected endpoints (401) while the public
// auth-health probe remains 200.
//
// Contract: auth-degradation-readonly/step2 Outcome "unlogged-user-readonly-entry"
func TestStep2_UnloggedUserReadonlyEntry(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step2-unlogged"}},
		Err:  errors.New("no session"), // no session at all → provider.Get returns error
	}
	h := watest.NewHarness(t, provider)

	watest.DegradeProbeB()

	// Protected read endpoint → 401 (no session).
	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w)

	// Public health probe → 200.
	wh := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/auth-health")
	watest.AssertOK(t, wh)
	assert.Contains(t, wh.Body.String(), `"write_disabled":true`)
}

// TestStep2_ProbeBRedisReadDegradation verifies the read cache fallback: under a
// Redis connection failure, a read endpoint with a primed session cache returns 200
// (cache hit); with no cache entry it returns 503 (cache miss).
//
// Contract: auth-degradation-readonly/step2 Outcome "probe-b-redis-read-degradation"
func TestStep2_ProbeBRedisReadDegradation(t *testing.T) {
	t.Run("cache-hit-200", func(t *testing.T) {
		sess := &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step2-cachehit"}}
		provider := &watest.FakeProvider{Sess: sess}
		h := watest.NewHarness(t, provider)
		h.SeedRoleBinding(t, 1001, models.RoleAdmin)

		// Prime the in-process session cache with one successful read.
		watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")

		// Now flip probe B to degraded and sever Redis.
		watest.DegradeProbeB()
		sess.GetErr = watest.ConnRefusedErr()

		// Read + cache hit → 200 (grace read fallback).
		w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
		watest.AssertOK(t, w)
	})

	t.Run("cache-miss-503", func(t *testing.T) {
		sess := &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step2-cachemiss"}}
		sess.GetErr = watest.ConnRefusedErr()
		provider := &watest.FakeProvider{Sess: sess}
		h := watest.NewHarness(t, provider)
		h.SeedRoleBinding(t, 1001, models.RoleAdmin)

		watest.DegradeProbeB()

		// Read + cache miss (SSID never primed) → 503.
		w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
		watest.AssertUnavailable(t, w)
	})
}
