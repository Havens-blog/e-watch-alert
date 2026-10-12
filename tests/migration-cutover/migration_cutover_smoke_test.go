// @feature watchalert-subproject-integration @api-functional
package migration_cutover

import (
	"net/http"
	"testing"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"

	"github.com/stretchr/testify/assert"
)

// TestMigrationCutoverSmoke is the happy-path smoke test for the migration-cutover
// journey: health check passes, config exports under a valid session, and config
// imports successfully.
func TestMigrationCutoverSmoke(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{ClaimsData: claimsFor(1001)},
	}
	h := watest.NewHarness(t, provider)
	registerHello(t, h)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	// 1. Health check.
	wHello := watest.DoRequest(t, h.Engine, http.MethodGet, "/hello")
	watest.AssertHTTPStatus(t, wHello, http.StatusOK)

	// 2. Export config with a valid session → 200.
	wExport := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/export/config")
	watest.AssertOK(t, wExport)
	assert.Contains(t, wExport.Body.String(), `"schema_version"`)

	// 3. Import config with a valid session → 200.
	wImport := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/import/config",
		watest.WithJSONBody(`{"config":{"schema_version":"watchalert/v1","rules":[],"datasources":[],"notifications":[]}}`))
	watest.AssertOK(t, wImport)
}
