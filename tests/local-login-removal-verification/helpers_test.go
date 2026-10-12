// @feature watchalert-subproject-integration @api-functional
package local_login_removal_verification

import (
	"testing"

	"watchAlert/internal/ctx"
	"watchAlert/internal/services"
	"watchAlert/tests/watest"
)

// newHarness creates a test harness and initializes the service layer so that
// full-router handlers (which call services.*Service) do not panic on nil.
func newHarness(t *testing.T, provider *watest.FakeProvider) *watest.Harness {
	t.Helper()
	h := watest.NewHarness(t, provider)
	services.NewServices(ctx.DO())
	return h
}
