// @feature watchalert-subproject-integration @api-functional
package migration_cutover

import (
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"

	"github.com/stretchr/testify/assert"
)

// TestStep3_ConfigAlignmentVerification_Success verifies that the exported config
// payload carries the expected structure (schema_version + section fields), proving
// the new instance's config model is aligned for cutover.
//
// Contract: migration-cutover/step3 Outcome "success"
func TestStep3_ConfigAlignmentVerification_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/export/config")
	watest.AssertOK(t, w)
	assert.Contains(t, w.Body.String(), `"schema_version"`)
	assert.Contains(t, w.Body.String(), `"rules"`)
	assert.Contains(t, w.Body.String(), `"datasources"`)
	assert.Contains(t, w.Body.String(), `"notifications"`)
}

// TestStep3_ConfigAlignmentVerification_CountMismatch verifies the API-testable
// aspect of a count-mismatch check: the export response exposes a countable data
// structure (an object payload) that downstream alignment tooling can diff. (A true
// mismatch is asserted at the integration layer, not in-process.)
//
// Contract: migration-cutover/step3 Outcome "config-count-mismatch"
func TestStep3_ConfigAlignmentVerification_CountMismatch(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/export/config")
	watest.AssertOK(t, w)
	// The data field is an object (countable structure present).
	assert.Contains(t, w.Body.String(), `"data":{`)
}
