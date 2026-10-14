// Package service implements local accounts and server-side sessions (ADR-003, v0.3.1): argon2id passwords,
// lockout and login rate limits, opaque session tokens (only SHA-256 stored), password changes that revoke other
// sessions, and audit records for every security-relevant step.
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	audit "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/auth/mfa"
	"github.com/agungharyanto/sentra-dcim/internal/auth/password"
	"github.com/agungharyanto/sentra-dcim/internal/auth/repo"
	"github.com/agungharyanto/sentra-dcim/internal/platform/config"
	"github.com/agungharyanto/sentra-dcim/internal/platform/principal"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

// Errors returned to handlers. Login failures are deliberately indistinguishable (ErrInvalidCredentials).
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrRateLimited        = errors.New("too many login attempts")
	ErrUnavailable        = errors.New("authentication temporarily unavailable")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrPolicy             = password.ErrPolicy
	ErrNotFound           = errors.New("not found")
	ErrInvalidUsername    = errors.New("username must be 3–64 characters: letters, digits, . _ -")
	ErrUsernameTaken      = errors.New("username already exists")
	ErrRolesMissing       = errors.New("roles are not seeded (run the migrations)")
	ErrInvalidMFACode     = errors.New("invalid authentication code")
	ErrMFAEnabled         = errors.New("MFA is already active")
	ErrMFANotEnrolled     = errors.New("MFA is not set up")
)

// Audit actions.
const (
	ActionLoginPassword  = "auth.login.password_ok" //nolint:gosec // audit action name; password correct, MFA pending
	ActionLoginSuccess   = "auth.login.success"     // password and MFA passed
	ActionMFAEnrol       = "auth.mfa.enrol"
	ActionMFAEnrolStart  = "auth.mfa.enrol_start"
	ActionMFAVerify      = "auth.mfa.verify"
	ActionMFARecovery    = "auth.mfa.recovery_codes"
	ActionLoginFailure   = "auth.login.failure"
	ActionLockout        = "auth.lockout"
	ActionLogout         = "auth.logout"
	ActionPasswordChange = "auth.password.change"
	ActionSessionRevoke  = "auth.session.revoke"
	ActionCreateAdmin    = "user.create_admin"
	ActionAuthzDenied    = "authz.denied"
	ActionCSRFDenied     = "security.csrf_denied"
)

// Limiter is a fixed-window counter (Redis in production).
type Limiter interface {
	// Hit counts one attempt for key and reports whether it is within limit, with the time until the window resets.
	Hit(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, retryAfter time.Duration, err error)
}

// RateLimitError carries the wait time of ErrRateLimited.
type RateLimitError struct{ RetryAfter time.Duration }

func (e *RateLimitError) Error() string { return ErrRateLimited.Error() }
func (e *RateLimitError) Unwrap() error { return ErrRateLimited }

// Querier is the subset of the generated repository used here.
type Querier = repo.Querier

// Service implements authentication.
type Service struct {
	q         Querier
	limiter   Limiter
	audit     *audit.Recorder
	log       *slog.Logger
	cfg       config.Auth
	params    password.Params
	dummyHash string
	now       func() time.Time
	mfa       *mfa.Keys
}

// New creates the service. limiter may be nil only for CLI use (no logins).
func New(q Querier, limiter Limiter, rec *audit.Recorder, log *slog.Logger, cfg config.Auth) (*Service, error) {
	p := password.Params{MemoryKiB: cfg.Argon2MemoryKiB, Time: cfg.Argon2Time, Threads: cfg.Argon2Threads}
	// Verified for unknown/locked/disabled accounts so every failed login costs the same time.
	dummy, err := password.Hash(randomString(24), p)
	if err != nil {
		return nil, err
	}
	keys, err := mfa.NewKeys(cfg.MFAKey)
	if err != nil {
		return nil, fmt.Errorf("MFA_ENC_KEY: %w", err)
	}
	return &Service{q: q, limiter: limiter, audit: rec, log: log, cfg: cfg, params: p, dummyHash: dummy, now: time.Now, mfa: keys}, nil
}

// User is the public view of an account.
type User struct {
	ID                 uuid.UUID `json:"id"`
	Username           string    `json:"username"`
	DisplayName        string    `json:"display_name"`
	MustChangePassword bool      `json:"must_change_password"`
	Roles              []string  `json:"roles"`
	Permissions        []string  `json:"permissions"`
	MFAEnabled         bool      `json:"mfa_enabled"`
}

// SessionInfo is the public view of a session.
type SessionInfo struct {
	ID            uuid.UUID `json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	LastSeenAt    time.Time `json:"last_seen_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	IdleExpiresAt time.Time `json:"idle_expires_at"`
	IP            *string   `json:"ip"`
	UserAgent     string    `json:"user_agent"`
	Current       bool      `json:"current"`
	// MFAVerified is false while the session is mfa_pending (only me, logout and auth/mfa/* are usable).
	MFAVerified bool       `json:"mfa_verified"`
	RevokedAt   *time.Time `json:"-"`
}

// Session is the authenticated state of a request.
type Session struct {
	Principal          principal.Principal
	User               User
	Info               SessionInfo
	MustChangePassword bool
	MFAVerified        bool
	MFAEnabled         bool
}

// LoginResult is returned on a successful login. Token and CSRF are only ever sent to the client as cookies.
type LoginResult struct {
	Token   string
	CSRF    string
	Session Session
}

var usernameRE = regexp.MustCompile(`^[A-Za-z0-9._-]{3,64}$`)

// Login authenticates username/password. oldToken (the client's current session cookie, if any) is revoked so a
// login always starts a fresh session (rotation).
func (s *Service) Login(ctx context.Context, username, pw, oldToken string, meta audit.Meta) (LoginResult, error) {
	username = strings.TrimSpace(username)
	fail := func(reason string, u *repo.User) error {
		ev := audit.Event{Action: ActionLoginFailure, Result: audit.ResultFailure, ActorUsername: username,
			Meta: meta, Metadata: map[string]any{"reason": reason}}
		if u != nil {
			id := u.ID
			ev.ActorID, ev.TargetType, ev.TargetID = &id, "user", u.ID.String()
		}
		_ = s.audit.Record(ctx, ev)
		return ErrInvalidCredentials
	}

	if err := s.rateLimit(ctx, username, meta); err != nil {
		if errors.Is(err, ErrRateLimited) {
			name := username
			if !usernameRE.MatchString(name) { // possibly a password typed into the username field: never store it
				name = ""
			}
			_ = s.audit.Record(ctx, audit.Event{Action: ActionLoginFailure, Result: audit.ResultDenied,
				ActorUsername: name, Meta: meta, Metadata: map[string]any{"reason": "rate_limited"}})
		}
		return LoginResult{}, err
	}

	if !usernameRE.MatchString(username) || len(pw) == 0 || len(pw) > 4*password.MaxLength {
		s.burn(pw)
		// A malformed "username" is often a password typed into the wrong field: never store it.
		if !usernameRE.MatchString(username) {
			username = ""
		}
		return LoginResult{}, fail("invalid_input", nil)
	}
	u, err := s.q.GetUserByUsername(ctx, username)
	if errors.Is(err, pgx.ErrNoRows) {
		s.burn(pw)
		return LoginResult{}, fail("unknown_user", nil)
	}
	if err != nil {
		return LoginResult{}, fmt.Errorf("get user: %w", err)
	}
	if u.Status != "active" {
		s.burn(pw)
		return LoginResult{}, fail("disabled", &u)
	}
	if u.LockedUntil.Valid && u.LockedUntil.Time.After(s.now()) {
		s.burn(pw)
		return LoginResult{}, fail("locked", &u)
	}

	ok, rehash, err := password.Verify(pw, u.PasswordHash, s.params)
	if err != nil {
		s.log.ErrorContext(ctx, "stored password hash unreadable", "user_id", u.ID)
		return LoginResult{}, fail("bad_hash", &u)
	}
	if !ok {
		row, err := s.q.RecordLoginFailure(ctx, repo.RecordLoginFailureParams{
			UserID: u.ID, Threshold: int32(s.cfg.LockThreshold), LockSeconds: s.cfg.LockDuration.Seconds(), //nolint:gosec // bounded in config
		})
		if err != nil {
			return LoginResult{}, fmt.Errorf("record failure: %w", err)
		}
		if int(row.FailedAttempts) == s.cfg.LockThreshold && row.LockedUntil.Valid {
			id := u.ID
			_ = s.audit.Record(ctx, audit.Event{Action: ActionLockout, Result: audit.ResultDenied, ActorUsername: u.Username,
				ActorID: &id, TargetType: "user", TargetID: u.ID.String(), Meta: meta,
				Metadata: map[string]any{"failed_attempts": row.FailedAttempts, "locked_until": row.LockedUntil.Time.UTC()}})
		}
		return LoginResult{}, fail("invalid_password", &u)
	}

	// The failure counter is reset only when MFA passes too (completeMFA): a known password alone must not
	// reset the lockout that also counts wrong MFA codes.
	if rehash {
		if h, err := password.Hash(pw, s.params); err == nil {
			if err := s.q.RehashPassword(ctx, repo.RehashPasswordParams{ID: u.ID, PasswordHash: h}); err != nil {
				s.log.WarnContext(ctx, "rehash failed", "user_id", u.ID, "error", err)
			}
		}
	}
	if oldToken != "" {
		s.revokeToken(ctx, oldToken)
	}
	res, err := s.newSession(ctx, u, meta)
	if err != nil {
		return LoginResult{}, err
	}
	id := u.ID
	_ = s.audit.Record(ctx, audit.Event{Action: ActionLoginPassword, Result: audit.ResultSuccess, ActorType: audit.ActorUser,
		ActorID: &id, ActorUsername: u.Username, TargetType: "session", TargetID: res.Session.Info.ID.String(), Meta: meta,
		Metadata: map[string]any{"mfa": map[bool]string{true: "verify", false: "enrol"}[res.Session.MFAEnabled]}})
	return res, nil
}

// rateLimit counts the attempt per IP and per username; Redis errors fail closed.
func (s *Service) rateLimit(ctx context.Context, username string, meta audit.Meta) error {
	return s.rateLimitKeys(ctx, "", username, meta)
}

// rateLimitKeys is rateLimit with a key prefix ("" for passwords, "mfa:" for MFA codes; same limits).
func (s *Service) rateLimitKeys(ctx context.Context, prefix, username string, meta audit.Meta) error {
	if s.limiter == nil {
		return ErrUnavailable
	}
	keys := []struct {
		key   string
		limit int
	}{{prefix + "user:" + hashKey(strings.ToLower(username)), s.cfg.RateUserLimit}}
	if meta.IP != nil {
		keys = append(keys, struct {
			key   string
			limit int
		}{prefix + "ip:" + meta.IP.String(), s.cfg.RateIPLimit})
	}
	var wait time.Duration
	for _, k := range keys {
		ok, retry, err := s.limiter.Hit(ctx, k.key, k.limit, s.cfg.RateWindow)
		if err != nil {
			s.log.ErrorContext(ctx, "login rate limiter unavailable", "error", err)
			return ErrUnavailable
		}
		if !ok && retry > wait {
			wait = retry
		}
	}
	if wait > 0 {
		return &RateLimitError{RetryAfter: wait}
	}
	return nil
}

// burn spends the same time as a real verification (timing-equalisation for failures that skip it).
func (s *Service) burn(pw string) { _, _, _ = password.Verify(pw, s.dummyHash, s.params) }

func (s *Service) newSession(ctx context.Context, u repo.User, meta audit.Meta) (LoginResult, error) {
	token, sum := newToken()
	now := s.now().UTC()
	sess, err := s.q.CreateSession(ctx, repo.CreateSessionParams{
		TokenHash:     sum,
		UserID:        u.ID,
		ExpiresAt:     ts(now.Add(s.cfg.SessionAbsolute)),
		IdleExpiresAt: ts(now.Add(s.cfg.SessionIdle)),
		Ip:            meta.IP,
		UserAgent:     meta.UserAgent,
	})
	if err != nil {
		return LoginResult{}, fmt.Errorf("create session: %w", err)
	}
	roles, err := s.q.GetUserRoleKeys(ctx, u.ID)
	if err != nil {
		return LoginResult{}, fmt.Errorf("get roles: %w", err)
	}
	enabled, err := s.mfaEnabled(ctx, u.ID)
	if err != nil {
		return LoginResult{}, err
	}
	p := newPrincipal(u.ID, u.Username, sess.ID, roles)
	user := newUser(u.ID, u.Username, u.DisplayName, u.MustChangePassword, p)
	user.MFAEnabled = enabled
	return LoginResult{
		Token: token,
		CSRF:  randomString(32),
		Session: Session{
			Principal:          p,
			User:               user,
			Info:               toInfo(sess.ID, sess.CreatedAt, sess.LastSeenAt, sess.ExpiresAt, sess.IdleExpiresAt, sess.Ip, sess.UserAgent, true),
			MustChangePassword: u.MustChangePassword,
			MFAEnabled:         enabled,
		},
	}, nil
}

// touchInterval throttles last_seen/idle updates to one write per minute per session.
const touchInterval = time.Minute

// Authenticate resolves a session token. Expired, idle, revoked or disabled → ErrUnauthenticated.
func (s *Service) Authenticate(ctx context.Context, token string) (Session, error) {
	if token == "" || len(token) > 128 {
		return Session{}, ErrUnauthenticated
	}
	row, err := s.q.GetActiveSession(ctx, sha(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrUnauthenticated
	}
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}
	now := s.now().UTC()
	idle := row.IdleExpiresAt.Time
	if now.Sub(row.LastSeenAt.Time) >= touchInterval {
		idle = now.Add(s.cfg.SessionIdle)
		if idle.After(row.ExpiresAt.Time) {
			idle = row.ExpiresAt.Time
		}
		if err := s.q.TouchSession(ctx, repo.TouchSessionParams{ID: row.SessionID, IdleExpiresAt: ts(idle)}); err != nil {
			s.log.WarnContext(ctx, "touch session failed", "error", err)
		}
	}
	p := newPrincipal(row.UserID, row.Username, row.SessionID, row.Roles)
	user := newUser(row.UserID, row.Username, row.DisplayName, row.MustChangePassword, p)
	user.MFAEnabled = row.MfaEnabled
	return Session{
		Principal: p,
		User:      user,
		Info: SessionInfo{ID: row.SessionID, CreatedAt: row.SessionCreatedAt.Time, LastSeenAt: now, ExpiresAt: row.ExpiresAt.Time,
			IdleExpiresAt: idle, Current: true, MFAVerified: row.MfaVerified},
		MustChangePassword: row.MustChangePassword,
		MFAVerified:        row.MfaVerified,
		MFAEnabled:         row.MfaEnabled,
	}, nil
}

// Logout revokes the current session.
func (s *Service) Logout(ctx context.Context, sess Session, meta audit.Meta) error {
	if _, err := s.q.RevokeSession(ctx, repo.RevokeSessionParams{ID: sess.Info.ID, UserID: sess.User.ID}); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	_ = s.audit.Record(ctx, audit.Event{Action: ActionLogout, Result: audit.ResultSuccess, TargetType: "session",
		TargetID: sess.Info.ID.String(), Meta: meta})
	return nil
}

// ChangePassword verifies the current password, stores the new one (clearing must_change_password) and revokes
// every other session of the user. The current session stays valid.
func (s *Service) ChangePassword(ctx context.Context, sess Session, current, next string, meta audit.Meta) error {
	failed := func(reason string, err error) error {
		_ = s.audit.Record(ctx, audit.Event{Action: ActionPasswordChange, Result: audit.ResultFailure, TargetType: "user",
			TargetID: sess.User.ID.String(), Meta: meta, Metadata: map[string]any{"reason": reason}})
		return err
	}
	u, err := s.q.GetUserByID(ctx, sess.User.ID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	ok, _, err := password.Verify(current, u.PasswordHash, s.params)
	if err != nil || !ok {
		return failed("wrong_current_password", ErrInvalidCredentials)
	}
	if err := password.CheckPolicy(next); err != nil {
		return failed("policy", ErrPolicy)
	}
	if next == current {
		return failed("unchanged", ErrPolicy)
	}
	h, err := password.Hash(next, s.params)
	if err != nil {
		return err
	}
	if err := s.q.UpdatePasswordHash(ctx, repo.UpdatePasswordHashParams{ID: u.ID, PasswordHash: h, MustChangePassword: false}); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	n, err := s.q.RevokeOtherSessions(ctx, repo.RevokeOtherSessionsParams{UserID: u.ID, KeepID: sess.Info.ID})
	if err != nil {
		return fmt.Errorf("revoke other sessions: %w", err)
	}
	_ = s.audit.Record(ctx, audit.Event{Action: ActionPasswordChange, Result: audit.ResultSuccess, TargetType: "user",
		TargetID: u.ID.String(), Meta: meta, Metadata: map[string]any{"other_sessions_revoked": n}})
	return nil
}

// ListSessions returns the user's active sessions, marking the current one.
func (s *Service) ListSessions(ctx context.Context, sess Session) ([]SessionInfo, error) {
	rows, err := s.q.ListActiveSessions(ctx, sess.User.ID)
	if err != nil {
		return nil, err
	}
	out := make([]SessionInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, toInfo(r.ID, r.CreatedAt, r.LastSeenAt, r.ExpiresAt, r.IdleExpiresAt, r.Ip, r.UserAgent, r.ID == sess.Info.ID))
	}
	return out, nil
}

// RevokeSession revokes one of the user's own sessions (ErrNotFound for other users' or unknown sessions).
func (s *Service) RevokeSession(ctx context.Context, sess Session, id uuid.UUID, meta audit.Meta) error {
	n, err := s.q.RevokeSession(ctx, repo.RevokeSessionParams{ID: id, UserID: sess.User.ID})
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	if n == 0 {
		_ = s.audit.Record(ctx, audit.Event{Action: ActionSessionRevoke, Result: audit.ResultFailure, TargetType: "session",
			TargetID: id.String(), Meta: meta, Metadata: map[string]any{"reason": "not_found"}})
		return ErrNotFound
	}
	_ = s.audit.Record(ctx, audit.Event{Action: ActionSessionRevoke, Result: audit.ResultSuccess, TargetType: "session",
		TargetID: id.String(), Meta: meta, Metadata: map[string]any{"current": id == sess.Info.ID}})
	return nil
}

// CreateAdmin creates the first/emergency administrator (CLI only) with the Admin role, in one statement. The
// password must be changed at first login.
func (s *Service) CreateAdmin(ctx context.Context, username, displayName, pw string) (User, error) {
	username = strings.TrimSpace(username)
	if !usernameRE.MatchString(username) {
		return User{}, ErrInvalidUsername
	}
	if err := password.CheckPolicy(pw); err != nil {
		return User{}, ErrPolicy
	}
	if strings.TrimSpace(displayName) == "" {
		displayName = username
	}
	h, err := password.Hash(pw, s.params)
	if err != nil {
		return User{}, err
	}
	u, err := s.q.CreateAdminUser(ctx, repo.CreateAdminUserParams{Username: username, DisplayName: displayName, PasswordHash: h})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		_ = s.audit.Record(ctx, audit.Event{Action: ActionCreateAdmin, Result: audit.ResultFailure, ActorType: audit.ActorSystem,
			ActorUsername: "cli", TargetType: "user", Metadata: map[string]any{"reason": "username_taken", "username": username}})
		return User{}, ErrUsernameTaken
	}
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	if u.RolesGranted != 1 {
		// The INSERT … SELECT found no admin role: the statement is one transaction, but the account exists.
		return User{}, ErrRolesMissing
	}
	_ = s.audit.Record(ctx, audit.Event{Action: ActionCreateAdmin, Result: audit.ResultSuccess, ActorType: audit.ActorSystem,
		ActorUsername: "cli", TargetType: "user", TargetID: u.ID.String(),
		Metadata: map[string]any{"username": u.Username, "roles": []string{rbac.RoleAdmin}}})
	p := newPrincipal(u.ID, u.Username, uuid.Nil, []string{rbac.RoleAdmin})
	return newUser(u.ID, u.Username, u.DisplayName, u.MustChangePassword, p), nil
}

// CleanupStale deletes sessions that expired, went idle or were revoked more than a day ago.
func (s *Service) CleanupStale(ctx context.Context) (int64, error) {
	return s.q.DeleteStaleSessions(ctx, ts(s.now().UTC().Add(-24*time.Hour)))
}

func (s *Service) revokeToken(ctx context.Context, token string) {
	if sess, err := s.Authenticate(ctx, token); err == nil {
		_, _ = s.q.RevokeSession(ctx, repo.RevokeSessionParams{ID: sess.Info.ID, UserID: sess.User.ID})
	}
}

// RecordDenied audits a request refused for a missing permission. Repeats by the same actor on the same route are
// folded into one authz.denied.repeated event per window (audit throttle).
func (s *Service) RecordDenied(ctx context.Context, perm rbac.Permission, route string, meta audit.Meta) {
	s.audit.RecordAggregated(ctx, actorKey(ctx, meta)+"|"+route, audit.Event{Action: ActionAuthzDenied,
		Result: audit.ResultDenied, TargetType: "route", TargetID: route, Meta: meta,
		Metadata: map[string]any{"permission": string(perm)}})
}

// RecordCSRFDenied audits an unsafe request refused by the CSRF check (aggregated like RecordDenied). origin is the
// client-supplied Origin/Referer origin (truncated), useful to spot cross-site attempts.
func (s *Service) RecordCSRFDenied(ctx context.Context, reason, route, origin string, meta audit.Meta) {
	if len(route) > 128 {
		route = route[:128]
	}
	if len(origin) > 128 {
		origin = origin[:128]
	}
	s.audit.RecordAggregated(ctx, actorKey(ctx, meta)+"|csrf|"+reason+"|"+route, audit.Event{Action: ActionCSRFDenied,
		Result: audit.ResultDenied, TargetType: "route", TargetID: route, Meta: meta,
		Metadata: map[string]any{"reason": reason, "origin": origin}})
}

// actorKey identifies who is being throttled: the signed-in user, else the client IP.
func actorKey(ctx context.Context, meta audit.Meta) string {
	if p, ok := principal.From(ctx); ok {
		return "u:" + p.ID.String()
	}
	if meta.IP != nil {
		return "ip:" + meta.IP.String()
	}
	return "anon"
}

func newPrincipal(id uuid.UUID, username string, sessionID uuid.UUID, roles []string) principal.Principal {
	if roles == nil {
		roles = []string{}
	}
	return principal.Principal{Type: principal.TypeUser, ID: id, Username: username, SessionID: sessionID,
		Roles: roles, Permissions: rbac.Resolve(roles)}
}

func newUser(id uuid.UUID, username, displayName string, mustChange bool, p principal.Principal) User {
	return User{ID: id, Username: username, DisplayName: displayName, MustChangePassword: mustChange,
		Roles: p.Roles, Permissions: rbac.Strings(p.Permissions)}
}

func toInfo(id uuid.UUID, created, seen, exp, idle pgtype.Timestamptz, ip *netip.Addr, ua string, current bool) SessionInfo {
	info := SessionInfo{ID: id, CreatedAt: created.Time, LastSeenAt: seen.Time, ExpiresAt: exp.Time, IdleExpiresAt: idle.Time,
		UserAgent: ua, Current: current}
	if ip != nil {
		v := ip.String()
		info.IP = &v
	}
	return info
}

// newToken returns a 256-bit random URL-safe token and its SHA-256 (the only form stored).
func newToken() (string, []byte) {
	t := randomString(32)
	return t, sha(t)
}

func sha(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }

func hashKey(s string) string { return base64.RawURLEncoding.EncodeToString(sha(s)[:16]) }

func randomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
