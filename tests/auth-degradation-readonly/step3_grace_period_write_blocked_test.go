// @feature watchalert-subproject-integration @api-functional
package auth_degradation_readonly

import (
	"errors"
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
)

const step3ValidBindingsBody = `{"bindings":[{"eiam_uid":2002,"role":"user"}]}`

// TestStep3_GracePeriodWriteBlocked_Success verifies that in degraded mode every
// write request is rejected with 503 regardless of session validity (the
// degradation guard short-circuits before session validation).
//
// Contract: auth-degradation-readonly/step3 Outcome "success"
func TestStep3_GracePeriodWriteBlocked_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step3-success"}},
	}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	watest.DegradeProbeB()

	w := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/role-bindings",
		watest.WithJSONBody(step3ValidBindingsBody))
	watest.AssertUnavailable(t, w)
}

// TestStep3_BypassFrontendDirectWriteAPI verifies that the 503 write block is
// enforced server-side: a direct POST to the import/config API (bypassing any
// frontend gate) is still rejected during degradation.
//
// Contract: auth-degradation-readonly/step3 Outcome "bypass-frontend-direct-write-api"
func TestStep3_BypassFrontendDirectWriteAPI(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step3-bypass"}},
	}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	watest.DegradeProbeB()

	w := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/import/config",
		watest.WithJSONBody(`{}`))
	watest.AssertUnavailable(t, w)
}

// TestStep3_UnauthNoSessionDuringDegradation verifies that an unauthenticated user
// hitting a READ endpoint during degradation is rejected with 401 (session check
// applies to reads; writes are 503'd earlier). This documents the code's actual
// ordering: the write-degradation guard precedes session validation, so for a read
// the session check still runs and a missing session yields 401.
//
// Contract: auth-degradation-readonly/step3 Outcome "unauth-no-session-during-degradation"
func TestStep3_UnauthNoSessionDuringDegradation(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step3-unauth"}},
		Err:  errors.New("no session"),
	}
	h := watest.NewHarness(t, provider)

	watest.DegradeProbeB()

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertUnauthorized(t, w)
}

// TestStep3_LegacyJWTWriteDuringDegradation verifies that a legacy JWT bearer cannot
// bypass the degradation write block: the degradation guard (step 2 of Auth) runs
// before the legacy-JWT fallback, so a write with a legacy token → 503.
//
// Contract: auth-degradation-readonly/step3 Outcome "legacy-jwt-write-during-degradation"
func TestStep3_LegacyJWTWriteDuringDegradation(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step3-legacy"}},
	}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	watest.DegradeProbeB()
	token := watest.GenerateLegacyJWT(t, "uid-legacy")

	w := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/role-bindings",
		watest.WithJSONBody(step3ValidBindingsBody), watest.WithAuthBearer(token))
	watest.AssertUnavailable(t, w)
}

// TestStep3_ProbeBRedisWriteBlocked verifies that probe-B-only degradation (Redis
// unreachable) blocks writes: a valid session + write → 503.
//
// Contract: auth-degradation-readonly/step3 Outcome "probe-b-redis-write-blocked"
func TestStep3_ProbeBRedisWriteBlocked(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-step3-redis"}},
	}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	watest.DegradeProbeB()

	w := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/import/config",
		watest.WithJSONBody(`{}`))
	watest.AssertUnavailable(t, w)
}
