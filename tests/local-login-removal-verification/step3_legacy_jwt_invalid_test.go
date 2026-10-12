// @feature watchalert-subproject-integration @api-functional
package local_login_removal_verification

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"watchAlert/pkg/tools"
	"watchAlert/tests/watest"

	"github.com/dgrijalva/jwt-go"
)

// forgedLegacyJWT signs a JWT with a wrong key to simulate a removed/rotated
// signing key (the VerifyOnlyKey has changed, so verification fails).
func forgedLegacyJWT(t *testing.T, uid string) string {
	t.Helper()
	claims := tools.JwtCustomClaims{
		ID:   uid,
		Name: "forged-user",
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
			IssuedAt:  time.Now().Unix(),
			Issuer:    "WatchAlert",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := token.SignedString([]byte("wrong-signing-key"))
	if err != nil {
		t.Fatalf("sign forged jwt: %v", err)
	}
	return "bearer " + s
}

// Contract: local-login-removal-verification/step3 Outcome "success"
// A forged legacy JWT (signed with a removed/rotated key) → 401, because
// verification fails and no shared session exists.
func TestStep3_LegacyJWTInvalid_ForgedTokenRejected(t *testing.T) {
	// provider.Err set → sessionProvider.Get returns error → tryLegacyJWT fallback
	provider := &watest.FakeProvider{Err: errors.New("no shared session")}
	h := newHarness(t, provider)

	token := forgedLegacyJWT(t, "uid-forged")
	w := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings",
		func(r *http.Request) { r.Header.Set("Authorization", token) })

	watest.AssertUnauthorized(t, w)
}

// Contract: local-login-removal-verification/step3 Outcome "legacy-jwt-transition-tolerance"
// A validly-signed legacy JWT is tolerated on read endpoints (200) but rejected
// on write endpoints (403), enforcing the write-path cutover to shared sessions.
func TestStep3_LegacyJWT_TransitionTolerance(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no shared session")}
	h := newHarness(t, provider)

	token := watest.GenerateLegacyJWT(t, "uid-legacy")

	// Read endpoint → tolerated (200)
	wRead := watest.DoRequest(t, h.Engine, http.MethodGet, "/api/watchalert/role-bindings",
		func(r *http.Request) { r.Header.Set("Authorization", token) })
	watest.AssertOK(t, wRead)

	// Write endpoint → rejected (403)
	wWrite := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/role-bindings",
		watest.WithJSONBody(`{"bindings":[]}`),
		func(r *http.Request) { r.Header.Set("Authorization", token) })
	watest.AssertForbidden(t, wWrite)
}

// Contract: local-login-removal-verification/step3 Outcome "legacy-jwt-write-rejected-403"
// A legacy JWT on a write endpoint → 403 (write path强制走共享会话).
func TestStep3_LegacyJWT_WriteRejected403(t *testing.T) {
	provider := &watest.FakeProvider{Err: errors.New("no shared session")}
	h := newHarness(t, provider)

	token := watest.GenerateLegacyJWT(t, "uid-legacy")
	w := watest.DoRequest(t, h.Engine, http.MethodPost, "/api/watchalert/role-bindings",
		watest.WithJSONBody(`{"bindings":[]}`),
		func(r *http.Request) { r.Header.Set("Authorization", token) })

	watest.AssertForbidden(t, w)
}
