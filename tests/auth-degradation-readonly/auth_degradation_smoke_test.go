// @feature watchalert-subproject-integration @api-functional
package auth_degradation_readonly

import (
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
)

// TestAuthDegradationSmoke is the happy-path smoke test for the auth-degradation
// journey: a valid session serves reads (200), degradation blocks writes (503),
// and recovery re-allows writes (200).
func TestAuthDegradationSmoke(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-smoke"}},
	}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	// 1. Valid session → read 200.
	wRead := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings")
	watest.AssertOK(t, wRead)

	// 2. Degrade → write 503.
	watest.DegradeProbeB()
	wWriteDegraded := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/role-bindings",
		watest.WithJSONBody(`{"bindings":[{"eiam_uid":2004,"role":"user"}]}`))
	watest.AssertUnavailable(t, wWriteDegraded)

	// 3. Recover → write 200.
	watest.RecoverProbeB()
	wWriteRecovered := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/role-bindings",
		watest.WithJSONBody(`{"bindings":[{"eiam_uid":2004,"role":"user"}]}`))
	watest.AssertOK(t, wWriteRecovered)
}
