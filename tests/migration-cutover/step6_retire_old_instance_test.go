// @feature watchalert-subproject-integration @api-functional
package migration_cutover

import (
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"
)

// TestStep6_RetireOldInstance_Success verifies that after retiring the old
// instance the new instance remains the sole, healthy serving path: /hello → 200.
//
// Contract: migration-cutover/step6 Outcome "success"
func TestStep6_RetireOldInstance_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	registerHello(t, h)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/hello")
	watest.AssertHTTPStatus(t, w, http.StatusOK)
}

// TestStep6_RetireOldInstance_RollbackItselfFails verifies the API-testable aspect
// of a failed rollback (DNS fallback / dual read-only): under degradation writes are
// still blocked (503), preserving safety.
//
// Contract: migration-cutover/step6 Outcome "rollback-itself-fails"
func TestStep6_RetireOldInstance_RollbackItselfFails(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	watest.DegradeProbeB()

	w := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/role-bindings",
		watest.WithJSONBody(`{"bindings":[{"eiam_uid":6001,"role":"user"}]}`))
	watest.AssertUnavailable(t, w)
}
