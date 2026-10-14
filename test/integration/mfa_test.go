//go:build integration

package integration

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/agungharyanto/sentra-dcim/internal/auth/mfa"
	"github.com/agungharyanto/sentra-dcim/internal/platform/config"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

func codeAt(secret []byte, offset int64) string { return mfa.Code(secret, mfa.Step(time.Now())+offset) }

func strList(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

// TestMFAEnrolmentAndPendingSession: after the password a session is mfa_pending and reaches nothing but me,
// logout and the MFA steps; enrolment needs one valid code and returns 10 recovery codes once.
func TestMFAEnrolmentAndPendingSession(t *testing.T) {
	e := newAuthEnv(t, nil)
	e.createUser("mfa.new", true)
	c := e.client()
	want(t, "password", c.loginPassword("mfa.new", pwInitial), http.StatusOK, "")
	me := c.get("/api/v1/auth/me")
	if me.body["session"].(map[string]any)["mfa_verified"] != false || me.body["user"].(map[string]any)["mfa_enabled"] != false {
		t.Fatalf("me while pending: %v", me.body)
	}
	for _, p := range []string{"/api/v1/ui/bootstrap", "/api/v1/help/index", "/api/v1/users", "/api/v1/auth/sessions"} {
		want(t, p+" while pending", c.get(p), http.StatusForbidden, "mfa_required")
	}
	want(t, "password change while pending", c.post("/api/v1/auth/password", map[string]string{"current_password": pwInitial, "new_password": pwNew}),
		http.StatusForbidden, "mfa_required")
	if v := c.get("/api/v1/version"); len(v.body) != 1 {
		t.Fatalf("/version details for a pending Admin session: %v", v.body)
	}
	want(t, "verify before enrolment", c.post("/api/v1/auth/mfa/verify", map[string]string{"code": "123456"}), http.StatusConflict, "mfa_not_enrolled")

	en := c.post("/api/v1/auth/mfa/enrol", nil)
	want(t, "enrol", en, http.StatusOK, "")
	if en.header.Get("Cache-Control") != "no-store" || !strings.HasPrefix(en.body["otpauth_uri"].(string), "otpauth://totp/Sentra%20DCIM:mfa.new?") ||
		len(en.body["qr"].([]any)) < 21 {
		t.Fatalf("enrolment response: %v", en.header)
	}
	secret := decodeSecret(t, en.body["secret"].(string))
	// Restarting the enrolment replaces the pending secret: the first one stops working.
	en2 := c.post("/api/v1/auth/mfa/enrol", nil)
	secret2 := decodeSecret(t, en2.body["secret"].(string))
	if bytes.Equal(secret, secret2) {
		t.Fatal("enrolment restart kept the secret")
	}
	want(t, "old pending secret", c.post("/api/v1/auth/mfa/enrol/confirm", map[string]string{"code": codeAt(secret, 0)}), http.StatusUnauthorized, "invalid_mfa_code")
	cr := c.post("/api/v1/auth/mfa/enrol/confirm", map[string]string{"code": codeAt(secret2, 0)})
	want(t, "confirm", cr, http.StatusOK, "")
	codes := strList(cr.body["recovery_codes"])
	if len(codes) != 10 || cr.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("recovery codes: %d", len(codes))
	}
	want(t, "bootstrap after MFA", c.get("/api/v1/ui/bootstrap"), http.StatusOK, "")
	want(t, "enrol again", c.post("/api/v1/auth/mfa/enrol", nil), http.StatusConflict, "mfa_already_enabled")
	st := c.get("/api/v1/auth/mfa")
	if st.body["enabled"] != true || st.body["recovery_codes_remaining"] != float64(10) {
		t.Fatalf("status %v", st.body)
	}

	// At rest: the secret is encrypted, recovery codes are keyed hashes; neither appears in the audit log.
	var sealed []byte
	_ = e.pool.QueryRow(context.Background(), `SELECT secret_enc FROM user_mfa m JOIN users u ON u.id = m.user_id WHERE u.username = 'mfa.new'`).Scan(&sealed)
	if len(sealed) == 0 || bytes.Contains(sealed, secret2) || bytes.Contains(sealed, []byte(mfa.EncodeSecret(secret2))) {
		t.Fatal("secret stored in clear")
	}
	var leaks int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE metadata::text LIKE '%' || $1 || '%' OR metadata::text LIKE '%' || $2 || '%'`,
		mfa.EncodeSecret(secret2)[:16], codes[0]).Scan(&leaks)
	if leaks != 0 {
		t.Fatal("secret or recovery code in the audit log")
	}
	for _, a := range []string{"auth.login.password_ok", "auth.mfa.enrol", "auth.login.success"} {
		if countAudit(t, e, a) == 0 {
			t.Errorf("no %s audit event", a)
		}
	}
}

// TestMFAReplayAndWindow: ±1 step accepted, a used step never again (also not from another session).
func TestMFAReplayAndWindow(t *testing.T) {
	e := newAuthEnv(t, nil)
	e.createUser("mfa.replay", true)
	c := e.client()
	want(t, "first", c.loginPassword("mfa.replay", pwInitial), http.StatusOK, "")
	secret := c.enrol() // used the current step

	b := e.client()
	want(t, "second password", b.loginPassword("mfa.replay", pwInitial), http.StatusOK, "")
	want(t, "replay current step", b.post("/api/v1/auth/mfa/verify", map[string]string{"code": codeAt(secret, 0)}), http.StatusUnauthorized, "invalid_mfa_code")
	want(t, "two steps ahead", b.post("/api/v1/auth/mfa/verify", map[string]string{"code": codeAt(secret, 2)}), http.StatusUnauthorized, "invalid_mfa_code")
	want(t, "next step (+1 skew)", b.post("/api/v1/auth/mfa/verify", map[string]string{"code": codeAt(secret, 1)}), http.StatusNoContent, "")
	cc := e.client()
	want(t, "third password", cc.loginPassword("mfa.replay", pwInitial), http.StatusOK, "")
	want(t, "replay next step", cc.post("/api/v1/auth/mfa/verify", map[string]string{"code": codeAt(secret, 1)}), http.StatusUnauthorized, "invalid_mfa_code")
	want(t, "previous step is older than the last used", cc.post("/api/v1/auth/mfa/verify", map[string]string{"code": codeAt(secret, -1)}), http.StatusUnauthorized, "invalid_mfa_code")
}

// TestMFARecoveryCodes: one-time use, normalised input, regeneration needs a current code and replaces all.
func TestMFARecoveryCodes(t *testing.T) {
	e := newAuthEnv(t, nil)
	e.createUser("mfa.rec", true)
	c := e.client()
	want(t, "password", c.loginPassword("mfa.rec", pwInitial), http.StatusOK, "")
	en := c.post("/api/v1/auth/mfa/enrol", nil)
	secret := decodeSecret(t, en.body["secret"].(string))
	codes := strList(c.post("/api/v1/auth/mfa/enrol/confirm", map[string]string{"code": codeAt(secret, 0)}).body["recovery_codes"])

	b := e.client()
	want(t, "password b", b.loginPassword("mfa.rec", pwInitial), http.StatusOK, "")
	typed := strings.ToUpper(strings.ReplaceAll(codes[0], "-", " "))
	want(t, "recovery code (typed differently)", b.post("/api/v1/auth/mfa/verify", map[string]string{"code": typed}), http.StatusNoContent, "")
	if st := b.get("/api/v1/auth/mfa"); st.body["recovery_codes_remaining"] != float64(9) {
		t.Fatalf("remaining %v", st.body)
	}
	d := e.client()
	want(t, "password d", d.loginPassword("mfa.rec", pwInitial), http.StatusOK, "")
	want(t, "used recovery code", d.post("/api/v1/auth/mfa/verify", map[string]string{"code": codes[0]}), http.StatusUnauthorized, "invalid_mfa_code")

	want(t, "regenerate without valid code", b.post("/api/v1/auth/mfa/recovery-codes", map[string]string{"code": "000000"}), http.StatusUnauthorized, "invalid_mfa_code")
	rg := b.post("/api/v1/auth/mfa/recovery-codes", map[string]string{"code": codeAt(secret, 1)})
	want(t, "regenerate", rg, http.StatusOK, "")
	fresh := strList(rg.body["recovery_codes"])
	if len(fresh) != 10 || slices.Contains(fresh, codes[1]) {
		t.Fatal("regenerated codes")
	}
	want(t, "old code after regeneration", d.post("/api/v1/auth/mfa/verify", map[string]string{"code": codes[1]}), http.StatusUnauthorized, "invalid_mfa_code")
	want(t, "new code", d.post("/api/v1/auth/mfa/verify", map[string]string{"code": fresh[0]}), http.StatusNoContent, "")
	want(t, "regenerate while pending is blocked", e.client().get("/api/v1/auth/mfa"), http.StatusUnauthorized, "unauthenticated")
}

// TestMFALockoutAndRateLimit: wrong codes count towards the login lockout (5 → locked, sessions revoked); MFA
// attempts are rate limited per account like logins.
func TestMFALockoutAndRateLimit(t *testing.T) {
	e := newAuthEnv(t, nil)
	e.createUser("mfa.lock", true)
	setup := e.client()
	want(t, "setup", setup.login("mfa.lock", pwInitial), http.StatusOK, "") // enrolled

	c := e.client()
	want(t, "password", c.loginPassword("mfa.lock", pwInitial), http.StatusOK, "")
	for i := range 5 {
		want(t, "wrong code "+string(rune('1'+i)), c.post("/api/v1/auth/mfa/verify", map[string]string{"code": "000000"}), http.StatusUnauthorized, "invalid_mfa_code")
	}
	want(t, "pending session revoked by lockout", c.get("/api/v1/auth/me"), http.StatusUnauthorized, "unauthenticated")
	want(t, "other sessions revoked too", setup.get("/api/v1/auth/me"), http.StatusUnauthorized, "unauthenticated")
	want(t, "password while locked", e.client().loginPassword("mfa.lock", pwInitial), http.StatusUnauthorized, "invalid_credentials")
	if countAudit(t, e, "auth.lockout") == 0 {
		t.Error("no lockout audit")
	}

	r := newAuthEnv(t, func(c *config.Auth) { c.RateUserLimit = 3; c.LockThreshold = 50 })
	r.createUser("mfa.rl", true)
	one := r.client()
	want(t, "setup rl", one.login("mfa.rl", pwInitial), http.StatusOK, "")
	two := r.client()
	want(t, "password rl", two.loginPassword("mfa.rl", pwInitial), http.StatusOK, "")
	got := 0
	for range 4 {
		got = two.post("/api/v1/auth/mfa/verify", map[string]string{"code": "000000"}).code
	}
	if got != http.StatusTooManyRequests {
		t.Fatalf("4th MFA attempt with limit 3 = %d, want 429", got)
	}
}

// TestMFAPasswordStepDoesNotResetLockout: a known password must not reset the failure counter that also counts
// wrong MFA codes (otherwise re-logging in would allow unlimited guessing).
func TestMFAPasswordStepDoesNotResetLockout(t *testing.T) {
	e := newAuthEnv(t, nil)
	e.createUser("mfa.count", true)
	want(t, "setup", e.client().login("mfa.count", pwInitial), http.StatusOK, "")
	for i := range 4 {
		c := e.client()
		want(t, "password", c.loginPassword("mfa.count", pwInitial), http.StatusOK, "")
		want(t, "wrong "+string(rune('1'+i)), c.post("/api/v1/auth/mfa/verify", map[string]string{"code": "000000"}), http.StatusUnauthorized, "invalid_mfa_code")
	}
	c := e.client()
	want(t, "password 5", c.loginPassword("mfa.count", pwInitial), http.StatusOK, "")
	want(t, "fifth wrong code locks", c.post("/api/v1/auth/mfa/verify", map[string]string{"code": "000000"}), http.StatusUnauthorized, "invalid_mfa_code")
	want(t, "locked", e.client().loginPassword("mfa.count", pwInitial), http.StatusUnauthorized, "invalid_credentials")
}

// TestMFAAdminReset: an Admin resets another user's MFA (sessions revoked, enrol again), never their own.
func TestMFAAdminReset(t *testing.T) {
	e := newAuthEnv(t, nil)
	admin := e.signIn("mfa.admin", rbac.RoleAdmin)
	user := e.signIn("mfa.user", rbac.RoleViewer)
	id := user.get("/api/v1/auth/me").body["user"].(map[string]any)["id"].(string)
	adminID := admin.get("/api/v1/auth/me").body["user"].(map[string]any)["id"].(string)
	if u := admin.get("/api/v1/users/" + id); u.body["mfa_enabled"] != true {
		t.Fatalf("mfa_enabled in user view: %v", u.body)
	}
	want(t, "viewer cannot reset", user.post("/api/v1/users/"+adminID+"/mfa-reset", nil), http.StatusForbidden, "forbidden")
	want(t, "reset own MFA", admin.post("/api/v1/users/"+adminID+"/mfa-reset", nil), http.StatusConflict, "self_action")
	want(t, "reset", admin.post("/api/v1/users/"+id+"/mfa-reset", nil), http.StatusNoContent, "")
	want(t, "user session revoked", user.get("/api/v1/auth/me"), http.StatusUnauthorized, "unauthenticated")
	again := e.client()
	r := again.loginPassword("mfa.user", pwInitial)
	if r.body["user"].(map[string]any)["mfa_enabled"] != false {
		t.Fatalf("after reset mfa_enabled = %v", r.body["user"])
	}
	again.enrol()
	var ok, denied int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FILTER (WHERE result = 'success'), count(*) FILTER (WHERE result = 'denied')
		FROM audit_events WHERE action = 'user.mfa.reset'`).Scan(&ok, &denied)
	if ok != 1 || denied != 1 {
		t.Errorf("user.mfa.reset audit: %d success, %d denied (want 1, 1)", ok, denied)
	}
}

// TestMFAMustChangeOrder: MFA comes first, then the forced password change.
func TestMFAMustChangeOrder(t *testing.T) {
	e := newAuthEnv(t, nil)
	e.createUser("mfa.first", false)
	c := e.client()
	want(t, "password", c.loginPassword("mfa.first", pwInitial), http.StatusOK, "")
	want(t, "password change before MFA", c.post("/api/v1/auth/password", map[string]string{"current_password": pwInitial, "new_password": pwNew}),
		http.StatusForbidden, "mfa_required")
	c.enrol()
	want(t, "bootstrap before password change", c.get("/api/v1/ui/bootstrap"), http.StatusForbidden, "password_change_required")
	want(t, "password change after MFA", c.post("/api/v1/auth/password", map[string]string{"current_password": pwInitial, "new_password": pwNew}),
		http.StatusNoContent, "")
	want(t, "bootstrap", c.get("/api/v1/ui/bootstrap"), http.StatusOK, "")
}

// TestCLIResetMFA: the emergency command resets any account, also the last Admin.
func TestCLIResetMFA(t *testing.T) {
	e := newAuthEnv(t, nil)
	admin := e.signIn("last.admin", rbac.RoleAdmin)
	bin := filepath.Join(t.TempDir(), "sentra")
	if out, err := exec.Command("go", "build", "-o", bin, "../../cmd/sentra").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	run := func(args ...string) (string, error) {
		cmd := exec.Command(bin, append([]string{"user", "reset-mfa"}, args...)...)
		cmd.Env = append(os.Environ(), cliEnv(t, e.appDSN)...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run("--username", "last.admin"); err != nil {
		t.Fatalf("reset-mfa: %v\n%s", err, out)
	}
	want(t, "admin session revoked", admin.get("/api/v1/auth/me"), http.StatusUnauthorized, "unauthenticated")
	if _, err := run("--username", "nobody.at.all"); err == nil {
		t.Fatal("unknown user accepted")
	}
	var actor string
	_ = e.pool.QueryRow(context.Background(), `SELECT actor_type || '/' || actor_username FROM audit_events WHERE action = 'user.mfa.reset'`).Scan(&actor)
	if actor != "system/cli" {
		t.Fatalf("audit actor = %q", actor)
	}
}

// TestCLIUnlock: a lockout ends early with the emergency command (otherwise after AUTH_LOCK_DURATION) and is audited.
func TestCLIUnlock(t *testing.T) {
	e := newAuthEnv(t, nil)
	e.createUser("locked.admin", true)
	c := e.client()
	for range 5 {
		c.loginPassword("locked.admin", "wrong-password-x")
	}
	want(t, "locked", c.loginPassword("locked.admin", pwInitial), http.StatusUnauthorized, "invalid_credentials")
	bin := filepath.Join(t.TempDir(), "sentra")
	if out, err := exec.Command("go", "build", "-o", bin, "../../cmd/sentra").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, "user", "unlock", "--username", "locked.admin")
	cmd.Env = append(os.Environ(), cliEnv(t, e.appDSN)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("unlock: %v\n%s", err, out)
	}
	want(t, "after unlock", e.client().loginPassword("locked.admin", pwInitial), http.StatusOK, "")
	if countAudit(t, e, "user.unlock") != 1 || countAudit(t, e, "auth.lockout") != 1 {
		t.Fatal("unlock/lockout audit events")
	}
}
