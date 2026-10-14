//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agungharyanto/sentra-dcim/internal/api"
	"github.com/agungharyanto/sentra-dcim/internal/audit/chain"
	audithandler "github.com/agungharyanto/sentra-dcim/internal/audit/handler"
	auditrepo "github.com/agungharyanto/sentra-dcim/internal/audit/repo"
	auditservice "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	authhandler "github.com/agungharyanto/sentra-dcim/internal/auth/handler"
	"github.com/agungharyanto/sentra-dcim/internal/auth/mfa"
	authrepo "github.com/agungharyanto/sentra-dcim/internal/auth/repo"
	authservice "github.com/agungharyanto/sentra-dcim/internal/auth/service"
	helphandler "github.com/agungharyanto/sentra-dcim/internal/help/handler"
	"github.com/agungharyanto/sentra-dcim/internal/platform/cache"
	"github.com/agungharyanto/sentra-dcim/internal/platform/config"
	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/platform/logging"
	"github.com/agungharyanto/sentra-dcim/internal/platform/migrator"
	settingshandler "github.com/agungharyanto/sentra-dcim/internal/settings/handler"
	settingsservice "github.com/agungharyanto/sentra-dcim/internal/settings/service"
	systemhandler "github.com/agungharyanto/sentra-dcim/internal/system/handler"
	systemservice "github.com/agungharyanto/sentra-dcim/internal/system/service"
	uihandler "github.com/agungharyanto/sentra-dcim/internal/ui/handler"
	uiservice "github.com/agungharyanto/sentra-dcim/internal/ui/service"
	usershandler "github.com/agungharyanto/sentra-dcim/internal/users/handler"
	usersservice "github.com/agungharyanto/sentra-dcim/internal/users/service"
)

// Test passwords and the MFA key are generated per run; they only exist in the throwaway database.
var (
	pwInitial  = "initial-" + randHex(8)
	pwNew      = "changed-" + randHex(8)
	testMFAKey = func() []byte { b := make([]byte, 32); _, _ = rand.Read(b); return b }()
)

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type authEnv struct {
	t    *testing.T
	srv  *httptest.Server
	pool *pgxpool.Pool
	svc  *authservice.Service
	usr  *usersservice.Service
	dsn  string
	// appDSN logs in as a test member of sentra_app (what the app and its CLI commands use in production).
	appDSN string
	// totp holds the MFA secret of each account enrolled through the API in this test (lower-case username).
	totp map[string][]byte
}

func testAuthConfig() config.Auth {
	return config.Auth{
		// Small argon2 parameters keep the suite fast; production defaults are covered by config tests.
		Argon2MemoryKiB: 8 * 1024, Argon2Time: 1, Argon2Threads: 1,
		SessionIdle: 30 * time.Minute, SessionAbsolute: 12 * time.Hour,
		LockThreshold: 5, LockDuration: 15 * time.Minute,
		RateIPLimit: 1000, RateUserLimit: 1000, RateWindow: 10 * time.Minute,
		MFAKey: testMFAKey, MFAIssuer: "Sentra DCIM",
	}
}

// newAuthEnv starts the full /api/v1 router (public listener variant) over TLS on a migrated throwaway database.
// Rate-limit keys use a random Redis prefix, so production counters are never touched.
func newAuthEnv(t *testing.T, mod func(*config.Auth)) *authEnv {
	t.Helper()
	addr := os.Getenv("SENTRA_IT_REDIS_ADDR")
	if addr == "" {
		t.Skip("SENTRA_IT_REDIS_ADDR not set")
	}
	ctx := context.Background()
	dsn := tempDB(t)
	m, err := migrator.New(dsn, logging.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = m.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	// The services run as the application role (DML only, ISS-039); pool (owner) is for test setup only.
	appDSN := appRoleDSN(t, dsn)
	appPool, err := pgxpool.New(ctx, appDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(appPool.Close)
	rdb, err := cache.Connect(ctx, config.Redis{Addr: addr, Password: os.Getenv("SENTRA_IT_REDIS_PASSWORD")})
	if err != nil {
		t.Fatal(err)
	}
	prefix := "sentra:it:" + randHex(6) + ":"
	t.Cleanup(func() {
		keys, _ := rdb.Keys(ctx, prefix+"*").Result()
		if len(keys) > 0 {
			_ = rdb.Del(ctx, keys...).Err()
		}
		_ = rdb.Close()
	})

	cfg := testAuthConfig()
	if mod != nil {
		mod(&cfg)
	}
	log := logging.Discard()
	rec := auditservice.New(auditrepo.New(appPool), log)
	svc, err := authservice.New(authrepo.New(appPool), authservice.NewRedisLimiter(rdb, prefix), rec, log, cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(nil)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	sys := systemservice.New(log, time.UTC)
	uiSvc := uiservice.New("id", []string{"id", "en"}, time.UTC)
	mods := api.Modules{
		System: systemhandler.New(sys),
		UI:     uihandler.New(uiSvc),
		Help:   helphandler.New(nil, "id", []string{"id", "en"}),
		Auth:   authhandler.New(svc, log, []string{srv.URL}),
	}
	usr := usersservice.New(appPool, rec, log, cfg)
	mods.Users = usershandler.New(usr, log)
	reg, err := settingsservice.NewRegistry(settingsservice.Defaults{Language: "id", Timezone: "Asia/Jakarta"})
	if err != nil {
		t.Fatal(err)
	}
	set := settingsservice.New(appPool, reg, rec, log)
	// Same wiring as cmd/sentra (serve).
	uiSvc.SetSystem(func(ctx context.Context) (string, string, bool) {
		r, err := set.SystemRegional(ctx)
		return r.Language, r.Timezone, err == nil
	})
	mods.Audit = audithandler.New(auditrepo.New(appPool), rec, chain.NewMonitor(appPool, log, nil, time.Hour), set.LocationFor, log)
	mods.Settings = settingshandler.New(set, log)
	mods.Auth.WithRegional(set)
	srv.Config.Handler = httpserver.NewRouter(log, nil, nil, api.Mount(mods, false))
	return &authEnv{t: t, srv: srv, pool: pool, svc: svc, usr: usr, dsn: dsn, appDSN: appDSN, totp: map[string][]byte{}}
}

// createUser creates an account through the CLI code path; ready=true clears must_change_password.
func (e *authEnv) createUser(name string, ready bool) {
	e.t.Helper()
	if _, err := e.svc.CreateAdmin(context.Background(), name, "", pwInitial); err != nil {
		e.t.Fatal(err)
	}
	if ready {
		e.exec("UPDATE users SET must_change_password = false WHERE username = $1", name)
	}
}

func (e *authEnv) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatalf("%s: %v", sql, err)
	}
}

type client struct {
	e   *authEnv
	hc  *http.Client
	url *url.URL
}

func (e *authEnv) client() *client {
	jar, _ := cookiejar.New(nil)
	// srv.Client() is shared: give every client (browser) its own cookie jar.
	hc := &http.Client{Transport: e.srv.Client().Transport, Jar: jar}
	u, _ := url.Parse(e.srv.URL)
	return &client{e: e, hc: hc, url: u}
}

func (c *client) cookie(name string) string {
	for _, ck := range c.hc.Jar.Cookies(c.url) {
		if ck.Name == name {
			return ck.Value
		}
	}
	return ""
}

type resp struct {
	code   int
	header http.Header
	body   map[string]any
}

func (r resp) errCode() string {
	if e, ok := r.body["error"].(map[string]any); ok {
		s, _ := e["code"].(string)
		return s
	}
	return ""
}

// reqOpt adjusts a request (e.g. drop the CSRF header, change Origin).
type reqOpt func(*http.Request)

func noCSRF(r *http.Request) { r.Header.Del(authhandler.CSRFHeader) }
func origin(o string) reqOpt { return func(r *http.Request) { r.Header.Set("Origin", o) } }
func csrfHeader(v string) reqOpt {
	return func(r *http.Request) { r.Header.Set(authhandler.CSRFHeader, v) }
}
func (c *client) get(p string) resp { return c.do(http.MethodGet, p, nil) }
func (c *client) del(p string) resp { return c.do(http.MethodDelete, p, nil) }
func (c *client) post(p string, body any, opts ...reqOpt) resp {
	return c.do(http.MethodPost, p, body, opts...)
}
func (c *client) put(p string, body any) resp   { return c.do(http.MethodPut, p, body) }
func (c *client) patch(p string, body any) resp { return c.do(http.MethodPatch, p, body) }

// do sends a browser-like request: same Origin and the double-submit CSRF header (cookie value, or a placeholder
// before login).
func (c *client) do(method, p string, body any, opts ...reqOpt) resp {
	c.e.t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, c.e.srv.URL+p, rd)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", c.e.srv.URL)
	tok := c.cookie(authhandler.CSRFCookie)
	if tok == "" {
		tok = "login"
	}
	req.Header.Set(authhandler.CSRFHeader, tok)
	for _, o := range opts {
		o(req)
	}
	res, err := c.hc.Do(req)
	if err != nil {
		c.e.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	out := resp{code: res.StatusCode, header: res.Header}
	_ = json.NewDecoder(res.Body).Decode(&out.body)
	return out
}

// loginPassword is only the password step: a 200 leaves the session mfa_pending.
func (c *client) loginPassword(user, pw string) resp {
	return c.post("/api/v1/auth/login", map[string]string{"username": user, "password": pw})
}

// login signs in completely like a user would: password, then MFA — enrolment on the account's first sign-in in
// this test, a TOTP code afterwards. Returns the response of the password step.
func (c *client) login(user, pw string) resp {
	r := c.loginPassword(user, pw)
	if r.code == http.StatusOK {
		c.completeMFA(strings.ToLower(user))
	}
	return r
}

// completeMFA passes the MFA step of the current pending session.
func (c *client) completeMFA(user string) {
	c.e.t.Helper()
	if secret, ok := c.e.totp[user]; ok {
		// Tests sign in the same account several times within one 30 s step: allow the current step again (the
		// anti-replay rule itself is covered by TestMFAReplayAndWindow).
		c.e.exec(`UPDATE user_mfa SET last_used_step = 0 WHERE user_id = (SELECT id FROM users WHERE username = $1)`, user)
		r := c.post("/api/v1/auth/mfa/verify", map[string]string{"code": mfa.Code(secret, mfa.Step(time.Now()))})
		if r.code == http.StatusNoContent {
			return
		}
		if r.errCode() != "mfa_not_enrolled" { // MFA was reset meanwhile: enrol again below
			c.e.t.Fatalf("mfa verify %s: %d %v", user, r.code, r.body)
		}
	}
	c.e.totp[user] = c.enrol()
}

// enrol runs the enrolment of the current pending session and returns the secret.
func (c *client) enrol() []byte {
	c.e.t.Helper()
	r := c.post("/api/v1/auth/mfa/enrol", nil)
	if r.code != http.StatusOK {
		c.e.t.Fatalf("mfa enrol: %d %v", r.code, r.body)
	}
	secret := decodeSecret(c.e.t, r.body["secret"].(string))
	cr := c.post("/api/v1/auth/mfa/enrol/confirm", map[string]string{"code": mfa.Code(secret, mfa.Step(time.Now()))})
	if cr.code != http.StatusOK {
		c.e.t.Fatalf("mfa confirm: %d %v", cr.code, cr.body)
	}
	return secret
}

// cliEnv is the environment of the sentra binary for the throwaway database dsn (same MFA key as the test server).
func cliEnv(t *testing.T, dsn string) []string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	pass, _ := u.User.Password()
	return []string{"DB_HOST=" + u.Hostname(), "DB_PORT=" + u.Port(), "DB_NAME=" + strings.TrimPrefix(u.Path, "/"),
		"DB_USER=" + u.User.Username(), "DB_PASSWORD=" + pass, "REDIS_PASSWORD=unused-by-cli",
		"AUTH_ARGON2_MEMORY_KIB=8192", "AUTH_ARGON2_TIME=1", "AUTH_ARGON2_THREADS=1",
		"MFA_ENC_KEY=" + base64.StdEncoding.EncodeToString(testMFAKey)}
}

func decodeSecret(t *testing.T, grouped string) []byte {
	t.Helper()
	b, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ReplaceAll(grouped, " ", ""))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func want(t *testing.T, what string, r resp, code int, errCode string) {
	t.Helper()
	if r.code != code || r.errCode() != errCode {
		t.Fatalf("%s: got %d %q %v, want %d %q", what, r.code, r.errCode(), r.body, code, errCode)
	}
}

func TestAuthLoginLogout(t *testing.T) {
	e := newAuthEnv(t, nil)
	e.createUser("operator.one", true)
	c := e.client()

	want(t, "me without session", c.get("/api/v1/auth/me"), 401, "unauthenticated")
	want(t, "bootstrap without session", c.get("/api/v1/ui/bootstrap"), 401, "unauthenticated")

	wrong := c.login("operator.one", "not-the-password")
	unknown := c.login("nobody.here", "not-the-password")
	want(t, "wrong password", wrong, 401, "invalid_credentials")
	want(t, "unknown user", unknown, 401, "invalid_credentials")
	if wrong.body["error"].(map[string]any)["message"] != unknown.body["error"].(map[string]any)["message"] {
		t.Fatal("login error messages differ between unknown user and wrong password")
	}

	ok := c.login("operator.one", pwInitial)
	want(t, "login", ok, 200, "")
	var session string
	for _, sc := range ok.header.Values("Set-Cookie") {
		if strings.HasPrefix(sc, authhandler.SessionCookie+"=") {
			session = sc
		}
	}
	for _, attr := range []string{"Path=/", "HttpOnly", "Secure", "SameSite=Strict"} {
		if !strings.Contains(session, attr) {
			t.Errorf("session cookie lacks %s: %s", attr, session)
		}
	}
	if strings.Contains(session, "Domain=") || strings.Contains(session, "Max-Age") {
		t.Errorf("session cookie must be host-only browser-session cookie: %s", session)
	}
	first := c.cookie(authhandler.SessionCookie)

	me := c.get("/api/v1/auth/me")
	want(t, "me", me, 200, "")
	if me.body["user"].(map[string]any)["username"] != "operator.one" {
		t.Fatalf("me = %v", me.body)
	}
	want(t, "bootstrap with session", c.get("/api/v1/ui/bootstrap"), 200, "")

	// Login again on the same client: the session is rotated, the old token is revoked.
	want(t, "re-login", c.login("operator.one", pwInitial), 200, "")
	if c.cookie(authhandler.SessionCookie) == first {
		t.Fatal("session token not rotated at login")
	}
	var revoked bool
	if err := e.pool.QueryRow(context.Background(),
		"SELECT revoked_at IS NOT NULL FROM sessions ORDER BY created_at LIMIT 1").Scan(&revoked); err != nil || !revoked {
		t.Fatalf("previous session not revoked (%v)", err)
	}

	r := c.post("/api/v1/auth/logout", nil)
	if r.code != 204 {
		t.Fatalf("logout = %d", r.code)
	}
	want(t, "me after logout", c.get("/api/v1/auth/me"), 401, "unauthenticated")

	// The raw token is never stored: only its SHA-256.
	var n int
	_ = e.pool.QueryRow(context.Background(), "SELECT count(*) FROM sessions WHERE encode(token_hash,'escape') = $1", first).Scan(&n)
	if n != 0 {
		t.Fatal("raw session token found in the database")
	}
}

func TestAuthCSRF(t *testing.T) {
	e := newAuthEnv(t, nil)
	e.createUser("operator.csrf", true)
	c := e.client()

	want(t, "login without CSRF header", c.post("/api/v1/auth/login",
		map[string]string{"username": "operator.csrf", "password": pwInitial}, noCSRF), 403, "csrf_failed")
	want(t, "login from foreign origin", c.post("/api/v1/auth/login",
		map[string]string{"username": "operator.csrf", "password": pwInitial}, origin("https://evil.example")), 403, "csrf_failed")
	want(t, "login", c.login("operator.csrf", pwInitial), 200, "")

	want(t, "logout without CSRF header", c.post("/api/v1/auth/logout", nil, noCSRF), 403, "csrf_failed")
	want(t, "logout with wrong CSRF token", c.post("/api/v1/auth/logout", nil, csrfHeader("forged")), 403, "csrf_failed")
	want(t, "logout from foreign origin", c.post("/api/v1/auth/logout", nil, origin("https://evil.example")), 403, "csrf_failed")
	want(t, "logout with null origin", c.post("/api/v1/auth/logout", nil, origin("null")), 403, "csrf_failed")
	want(t, "still logged in", c.get("/api/v1/auth/me"), 200, "")
	if r := c.post("/api/v1/auth/logout", nil); r.code != 204 {
		t.Fatalf("logout with valid CSRF = %d", r.code)
	}
}

func TestAuthLockoutAndUnlock(t *testing.T) {
	e := newAuthEnv(t, func(c *config.Auth) { c.LockDuration = 2 * time.Second })
	e.createUser("operator.lock", true)
	c := e.client()
	for i := range 5 {
		want(t, "wrong password "+string(rune('1'+i)), c.login("operator.lock", "wrong-password-x"), 401, "invalid_credentials")
	}
	// Locked: even the right password gets the same generic answer.
	want(t, "correct password while locked", c.login("operator.lock", pwInitial), 401, "invalid_credentials")
	var locked bool
	_ = e.pool.QueryRow(context.Background(), "SELECT locked_until > now() FROM users WHERE username='operator.lock'").Scan(&locked)
	if !locked {
		t.Fatal("account not locked after 5 failures")
	}
	time.Sleep(2200 * time.Millisecond)
	want(t, "login after lock expired", c.login("operator.lock", pwInitial), 200, "")
	var attempts int
	_ = e.pool.QueryRow(context.Background(), "SELECT failed_attempts FROM users WHERE username='operator.lock'").Scan(&attempts)
	if attempts != 0 {
		t.Fatalf("failed_attempts = %d after success", attempts)
	}
	if n := countAudit(t, e, "auth.lockout"); n != 1 {
		t.Fatalf("auth.lockout events = %d", n)
	}
}

func TestAuthRateLimit(t *testing.T) {
	t.Run("per username", func(t *testing.T) {
		e := newAuthEnv(t, func(c *config.Auth) { c.RateUserLimit = 3 })
		c := e.client()
		for range 3 {
			want(t, "attempt", c.login("someone.rl", "wrong-password-x"), 401, "invalid_credentials")
		}
		r := c.login("someone.rl", "wrong-password-x")
		want(t, "over the username limit", r, 429, "rate_limited")
		if r.header.Get("Retry-After") == "" {
			t.Fatal("429 without Retry-After")
		}
		want(t, "other username not limited", c.login("other.rl", "wrong-password-x"), 401, "invalid_credentials")
	})
	t.Run("per IP", func(t *testing.T) {
		e := newAuthEnv(t, func(c *config.Auth) { c.RateIPLimit = 2 })
		c := e.client()
		want(t, "1", c.login("user.a", "wrong-password-x"), 401, "invalid_credentials")
		want(t, "2", c.login("user.b", "wrong-password-x"), 401, "invalid_credentials")
		want(t, "3 from same IP", c.login("user.c", "wrong-password-x"), 429, "rate_limited")
	})
}

func TestAuthIdleAndAbsoluteTimeout(t *testing.T) {
	t.Run("idle", func(t *testing.T) {
		e := newAuthEnv(t, func(c *config.Auth) { c.SessionIdle = time.Second })
		e.createUser("operator.idle", true)
		c := e.client()
		want(t, "login", c.login("operator.idle", pwInitial), 200, "")
		want(t, "me", c.get("/api/v1/auth/me"), 200, "")
		time.Sleep(1300 * time.Millisecond)
		want(t, "me after idle timeout", c.get("/api/v1/auth/me"), 401, "unauthenticated")
	})
	t.Run("absolute", func(t *testing.T) {
		e := newAuthEnv(t, nil)
		e.createUser("operator.abs", true)
		c := e.client()
		want(t, "login", c.login("operator.abs", pwInitial), 200, "")
		var idle time.Time
		_ = e.pool.QueryRow(context.Background(), "SELECT idle_expires_at FROM sessions").Scan(&idle)
		// Idle limit still far away; only the absolute lifetime has passed.
		e.exec("UPDATE sessions SET expires_at = now() - interval '1 second'")
		if time.Until(idle) < 29*time.Minute {
			t.Fatalf("idle expiry unexpectedly close: %s", idle)
		}
		want(t, "me after absolute timeout", c.get("/api/v1/auth/me"), 401, "unauthenticated")
	})
}

func TestAuthSessionsRevokeAndPasswordChange(t *testing.T) {
	e := newAuthEnv(t, nil)
	e.createUser("operator.multi", true)
	e.createUser("operator.other", true)
	a, b, other := e.client(), e.client(), e.client()
	want(t, "login a", a.login("operator.multi", pwInitial), 200, "")
	want(t, "login b", b.login("operator.multi", pwInitial), 200, "")
	want(t, "login other", other.login("operator.other", pwInitial), 200, "")

	list := a.get("/api/v1/auth/sessions")
	want(t, "list", list, 200, "")
	sessions := list.body["sessions"].([]any)
	if len(sessions) != 2 {
		t.Fatalf("sessions = %d, want 2 (own only)", len(sessions))
	}
	var bID string
	for _, s := range sessions {
		m := s.(map[string]any)
		if m["current"] != true {
			bID = m["id"].(string)
		}
	}
	otherID := other.get("/api/v1/auth/sessions").body["sessions"].([]any)[0].(map[string]any)["id"].(string)
	want(t, "revoke another user's session", a.del("/api/v1/auth/sessions/"+otherID), 404, "not_found")
	want(t, "other still logged in", other.get("/api/v1/auth/me"), 200, "")

	if r := a.del("/api/v1/auth/sessions/" + bID); r.code != 204 {
		t.Fatalf("revoke own session = %d", r.code)
	}
	want(t, "b after revoke", b.get("/api/v1/auth/me"), 401, "unauthenticated")

	// Password change revokes every other session, keeps the current one.
	want(t, "login b again", b.login("operator.multi", pwInitial), 200, "")
	want(t, "wrong current password", a.post("/api/v1/auth/password",
		map[string]string{"current_password": "wrong-password-x", "new_password": pwNew}), 400, "invalid_current_password")
	want(t, "too short", a.post("/api/v1/auth/password",
		map[string]string{"current_password": pwInitial, "new_password": "short"}), 422, "password_policy")
	want(t, "unchanged", a.post("/api/v1/auth/password",
		map[string]string{"current_password": pwInitial, "new_password": pwInitial}), 422, "password_policy")
	if r := a.post("/api/v1/auth/password", map[string]string{"current_password": pwInitial, "new_password": pwNew}); r.code != 204 {
		t.Fatalf("change password = %d %v", r.code, r.body)
	}
	want(t, "a keeps its session", a.get("/api/v1/auth/me"), 200, "")
	want(t, "b revoked by password change", b.get("/api/v1/auth/me"), 401, "unauthenticated")
	want(t, "old password", e.client().login("operator.multi", pwInitial), 401, "invalid_credentials")
	want(t, "new password", e.client().login("operator.multi", pwNew), 200, "")
}

func TestAuthMustChangePassword(t *testing.T) {
	e := newAuthEnv(t, nil)
	e.createUser("first.admin", false)
	c := e.client()
	r := c.login("first.admin", pwInitial)
	want(t, "login", r, 200, "")
	if r.body["user"].(map[string]any)["must_change_password"] != true {
		t.Fatalf("login response lacks must_change_password: %v", r.body)
	}
	want(t, "bootstrap blocked", c.get("/api/v1/ui/bootstrap"), 403, "password_change_required")
	want(t, "sessions blocked", c.get("/api/v1/auth/sessions"), 403, "password_change_required")
	want(t, "help blocked", c.get("/api/v1/help/index"), 403, "password_change_required")
	want(t, "me allowed", c.get("/api/v1/auth/me"), 200, "")
	if r := c.post("/api/v1/auth/password", map[string]string{"current_password": pwInitial, "new_password": pwNew}); r.code != 204 {
		t.Fatalf("change password = %d %v", r.code, r.body)
	}
	want(t, "bootstrap after change", c.get("/api/v1/ui/bootstrap"), 200, "")
	if c.get("/api/v1/auth/me").body["user"].(map[string]any)["must_change_password"] != false {
		t.Fatal("must_change_password still set")
	}
}

func countAudit(t *testing.T, e *authEnv, action string) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), "SELECT count(*) FROM audit_events WHERE action=$1", action).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAuthAuditTrail(t *testing.T) {
	e := newAuthEnv(t, nil)
	e.createUser("operator.audit", true)
	a, b := e.client(), e.client()
	want(t, "failure", a.login("operator.audit", "wrong-password-x"), 401, "invalid_credentials")
	want(t, "login a", a.login("operator.audit", pwInitial), 200, "")
	want(t, "login b", b.login("operator.audit", pwInitial), 200, "")
	bID := ""
	for _, s := range a.get("/api/v1/auth/sessions").body["sessions"].([]any) {
		if m := s.(map[string]any); m["current"] != true {
			bID = m["id"].(string)
		}
	}
	a.del("/api/v1/auth/sessions/" + bID)
	a.post("/api/v1/auth/password", map[string]string{"current_password": pwInitial, "new_password": pwNew})
	a.post("/api/v1/auth/logout", nil)

	for action, n := range map[string]int{
		"user.create_admin": 1, "auth.login.failure": 1, "auth.login.success": 2,
		"auth.session.revoke": 1, "auth.password.change": 1, "auth.logout": 1,
	} {
		if got := countAudit(t, e, action); got != n {
			t.Errorf("audit %s = %d, want %d", action, got, n)
		}
	}
	var withActor int
	_ = e.pool.QueryRow(context.Background(),
		"SELECT count(*) FROM audit_events WHERE action='auth.logout' AND actor_type='user' AND actor_username='operator.audit' AND ip IS NOT NULL AND request_id <> ''").Scan(&withActor)
	if withActor != 1 {
		t.Error("logout event lacks actor/ip/request_id")
	}

	// Secrets never reach the audit log: no password, token or hash in any column.
	var dump string
	if err := e.pool.QueryRow(context.Background(), "SELECT coalesce(string_agg(t::text, E'\\n'), '') FROM audit_events t").Scan(&dump); err != nil {
		t.Fatal(err)
	}
	token := a.cookie(authhandler.SessionCookie)
	for _, secret := range []string{pwInitial, pwNew, "wrong-password-x", "$argon2id$"} {
		if strings.Contains(dump, secret) {
			t.Fatalf("audit log contains a secret (%.4s…)", secret)
		}
	}
	if token != "" && strings.Contains(dump, token) {
		t.Fatal("audit log contains a session token")
	}

	// Append-only: UPDATE, DELETE and TRUNCATE are rejected by triggers.
	ctx := context.Background()
	for _, sql := range []string{
		"UPDATE audit_events SET action='tampered'",
		"DELETE FROM audit_events",
		"TRUNCATE audit_events",
	} {
		if _, err := e.pool.Exec(ctx, sql); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("%s: want append-only error, got %v", sql, err)
		}
	}
}

// TestCLICreateAdmin builds the real binary and runs `sentra user create-admin` with the password on stdin.
func TestCLICreateAdmin(t *testing.T) {
	if os.Getenv("SENTRA_IT_REDIS_ADDR") == "" {
		t.Skip("SENTRA_IT_REDIS_ADDR not set")
	}
	dsn := tempDB(t)
	m, err := migrator.New(dsn, logging.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	_ = m.Close()

	bin := filepath.Join(t.TempDir(), "sentra")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/sentra")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	env := append(os.Environ(), cliEnv(t, dsn)...)
	run := func(stdin string, args ...string) (string, error) {
		cmd := exec.Command(bin, append([]string{"user", "create-admin"}, args...)...)
		cmd.Env = env
		cmd.Stdin = strings.NewReader(stdin)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	out, err := run(pwInitial+"\n", "--username", "199001012020011001", "--display-name", "Admin Pertama")
	if err != nil {
		t.Fatalf("create-admin: %v\n%s", err, out)
	}
	if strings.Contains(out, pwInitial) {
		t.Fatal("CLI output echoes the password")
	}
	if _, err := run(pwInitial+"\n", "--username", "199001012020011001"); err == nil {
		t.Fatal("duplicate username accepted")
	}
	if _, err := run("short\n", "--username", "second.admin"); err == nil {
		t.Fatal("password shorter than 12 accepted")
	}
	if _, err := run(pwInitial+"\n", "--username", "bad name!"); err == nil {
		t.Fatal("invalid username accepted")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var must bool
	var display, hash string
	if err := pool.QueryRow(context.Background(),
		"SELECT must_change_password, display_name, password_hash FROM users WHERE username='199001012020011001'").Scan(&must, &display, &hash); err != nil {
		t.Fatal(err)
	}
	if !must || display != "Admin Pertama" || !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("user: must_change=%t display=%q hash prefix ok=%t", must, display, strings.HasPrefix(hash, "$argon2id$"))
	}
	var users, events int
	_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM users").Scan(&users)
	_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM audit_events WHERE action='user.create_admin' AND actor_type='system'").Scan(&events)
	if users != 1 || events < 1 {
		t.Fatalf("users=%d create_admin events=%d", users, events)
	}
	var roles []string
	if err := pool.QueryRow(context.Background(), `SELECT array_agg(r.key) FROM user_roles ur JOIN roles r ON r.id = ur.role_id
		JOIN users u ON u.id = ur.user_id WHERE u.username = '199001012020011001'`).Scan(&roles); err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0] != "admin" {
		t.Fatalf("create-admin roles = %v, want [admin]", roles)
	}
}
