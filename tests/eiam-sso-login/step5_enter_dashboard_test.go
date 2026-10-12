// @feature watchalert-subproject-integration @api-functional
package eiam_sso_login

import (
	"net/http"
	"testing"
	"time"

	"watchAlert/internal/models"
	"watchAlert/tests/watest"

	"github.com/ecodeclub/ginx/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Contract: eiam-sso-login/step5 Outcome "success"
// Authenticated + admin role → dashboard accessible (GET /api/system/userInfo → 200).
func TestStep5_EnterDashboard_Success(t *testing.T) {
	provider := &watest.FakeProvider{
		Sess: &watest.FakeSession{
			ClaimsData: session.Claims{Uid: 1001, SSID: "ssid-1"},
		},
	}
	h := newHarness(t, provider)
	h.SeedRoleBinding(t, 1001, models.RoleAdmin)

	// Seed an admin user so GetUserInfo finds the record.
	// Auth middleware maps admin role → UserId "admin".
	require.NoError(t, h.Repo.User().Create(models.Member{
		UserId:   "admin",
		UserName: "admin",
		CreateBy: "test",
		CreateAt: time.Now().Unix(),
	}))

	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/system/userInfo")
	assert.Equal(t, http.StatusOK, w.Code)
}
