// Package service implements user administration (v0.3.2, ADR-003, D-058): accounts, role assignment,
// disabling/enabling and temporary-password resets. Every change is audited; temporary passwords are returned to
// the caller exactly once and never logged or audited.
package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	audit "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/auth/password"
	"github.com/agungharyanto/sentra-dcim/internal/platform/config"
	"github.com/agungharyanto/sentra-dcim/internal/platform/principal"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
	"github.com/agungharyanto/sentra-dcim/internal/users/repo"
)

// Errors returned to the handler.
var (
	ErrNotFound      = errors.New("user not found")
	ErrUsernameTaken = errors.New("username already exists")
	ErrLastAdmin     = errors.New("the last active Admin cannot lose the Admin role or be disabled")
	ErrSelf          = errors.New("this action is not allowed on your own account")
)

// ValidationError names the invalid input field.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Reason }

// Statuses.
const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// Audit actions.
const (
	ActionCreate        = "user.create"
	ActionUpdate        = "user.update"
	ActionRolesUpdate   = "user.roles.update"
	ActionDisable       = "user.disable"
	ActionEnable        = "user.enable"
	ActionPasswordReset = "user.password.reset"
	ActionMFAReset      = "user.mfa.reset"
	ActionUnlock        = "user.unlock"
)

// Limits.
const (
	MaxDisplayName = 128
	MaxEmail       = 254
	MaxRoles       = 16
	DefaultLimit   = 50
	MaxLimit       = 200
)

var (
	usernameRE = regexp.MustCompile(`^[A-Za-z0-9._-]{3,64}$`) // same CHECK as users.username
	emailRE    = regexp.MustCompile(`^[^@\s]+@[^@\s]+$`)      // same CHECK as users.email
)

// DB is a pgx pool (queries + transactions).
type DB interface {
	repo.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Service implements user administration.
type Service struct {
	db     DB
	q      *repo.Queries
	audit  *audit.Recorder
	log    *slog.Logger
	params password.Params
}

// New creates the service; argon2 parameters come from the auth configuration.
func New(db DB, rec *audit.Recorder, log *slog.Logger, cfg config.Auth) *Service {
	return &Service{db: db, q: repo.New(db), audit: rec, log: log,
		params: password.Params{MemoryKiB: cfg.Argon2MemoryKiB, Time: cfg.Argon2Time, Threads: cfg.Argon2Threads}}
}

// User is the administrative view of an account (never the password hash).
type User struct {
	ID                 uuid.UUID  `json:"id"`
	Username           string     `json:"username"`
	DisplayName        string     `json:"display_name"`
	Email              *string    `json:"email"`
	Status             string     `json:"status"`
	Roles              []string   `json:"roles"`
	MustChangePassword bool       `json:"must_change_password"`
	Locked             bool       `json:"locked"`
	MFAEnabled         bool       `json:"mfa_enabled"`
	LastLoginAt        *time.Time `json:"last_login_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// Role is a role with the permissions it grants.
type Role struct {
	Key         string   `json:"key"`
	System      bool     `json:"system"`
	Permissions []string `json:"permissions"`
	UserCount   int64    `json:"user_count"`
}

// Filter selects users for List.
type Filter struct {
	Query  string
	Status string
	Role   string
	Limit  int
	Offset int
}

// List returns one page of users ordered by username, and the total number of matches.
func (s *Service) List(ctx context.Context, f Filter) ([]User, int64, error) {
	if f.Status != "" && f.Status != StatusActive && f.Status != StatusDisabled {
		return nil, 0, &ValidationError{Field: "status", Reason: "must be active or disabled"}
	}
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit || f.Offset < 0 || utf8.RuneCountInString(f.Query) > 64 {
		return nil, 0, &ValidationError{Field: "limit", Reason: "limit 1–200, offset ≥ 0, q ≤ 64 characters"}
	}
	rows, err := s.q.ListUsers(ctx, repo.ListUsersParams{
		Query: likeEscape(strings.TrimSpace(f.Query)), Status: f.Status, Role: f.Role,
		Lim: int32(f.Limit), Off: int32(f.Offset), //nolint:gosec // bounded above
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	out := make([]User, 0, len(rows))
	var total int64
	for _, r := range rows {
		out = append(out, view(repo.GetUserRow{ID: r.ID, Username: r.Username, DisplayName: r.DisplayName, Email: r.Email,
			Status: r.Status, MustChangePassword: r.MustChangePassword, Locked: r.Locked, LastLoginAt: r.LastLoginAt,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Roles: r.Roles, MfaEnabled: r.MfaEnabled}))
		total = r.Total
	}
	return out, total, nil
}

// Get returns one user.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (User, error) {
	return s.get(ctx, s.q, id)
}

// Roles lists the roles with their permissions (from code) and number of users.
func (s *Service) Roles(ctx context.Context) ([]Role, error) {
	rows, err := s.q.ListRoles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	byKey := make(map[string]repo.ListRolesRow, len(rows))
	for _, r := range rows {
		byKey[r.Key] = r
	}
	out := make([]Role, 0, len(rows))
	// System roles first in their display order, then any others alphabetically.
	for _, k := range rbac.SystemRoleKeys {
		if r, ok := byKey[k]; ok {
			out = append(out, Role{Key: k, System: r.System, Permissions: rbac.Strings(rbac.RolePermissions(k)), UserCount: r.UserCount})
			delete(byKey, k)
		}
	}
	for _, r := range rows {
		if _, ok := byKey[r.Key]; ok {
			out = append(out, Role{Key: r.Key, System: r.System, Permissions: []string{}, UserCount: r.UserCount})
		}
	}
	return out, nil
}

// CreateInput is a new account.
type CreateInput struct {
	Username    string
	DisplayName string
	Email       string
	Roles       []string
}

// Create creates an account with a generated temporary password (returned once; must be changed at first login).
func (s *Service) Create(ctx context.Context, in CreateInput, meta audit.Meta) (User, string, error) {
	in.Username = strings.TrimSpace(in.Username)
	if !usernameRE.MatchString(in.Username) {
		return User{}, "", &ValidationError{Field: "username", Reason: "3–64 characters: letters, digits, . _ -"}
	}
	name, err := checkDisplayName(in.DisplayName)
	if err != nil {
		return User{}, "", err
	}
	email, err := checkEmail(in.Email)
	if err != nil {
		return User{}, "", err
	}
	temp, hash, err := s.tempPassword()
	if err != nil {
		return User{}, "", err
	}
	actor := actorID(ctx)
	var u User
	err = s.tx(ctx, func(q *repo.Queries) error {
		roleIDs, err := resolveRoles(ctx, q, in.Roles)
		if err != nil {
			return err
		}
		id, err := q.CreateUser(ctx, repo.CreateUserParams{Username: in.Username, DisplayName: name, Email: email, PasswordHash: hash})
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrUsernameTaken
		}
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		for _, rid := range roleIDs {
			if err := q.AddUserRole(ctx, repo.AddUserRoleParams{UserID: id, RoleID: rid, GrantedBy: actor}); err != nil {
				return fmt.Errorf("add role: %w", err)
			}
		}
		u, err = s.get(ctx, q, id)
		return err
	})
	if err != nil {
		if errors.Is(err, ErrUsernameTaken) {
			s.record(ctx, ActionCreate, audit.ResultFailure, "", meta, map[string]any{"reason": "username_taken", "username": in.Username})
		}
		return User{}, "", err
	}
	s.record(ctx, ActionCreate, audit.ResultSuccess, u.ID.String(), meta, map[string]any{"username": u.Username, "roles": u.Roles})
	return u, temp, nil
}

// ProfileInput changes display name and/or e-mail (nil = unchanged; empty e-mail = remove).
type ProfileInput struct {
	DisplayName *string
	Email       *string
}

// UpdateProfile changes the display name and/or e-mail. The username is immutable (audit references).
func (s *Service) UpdateProfile(ctx context.Context, id uuid.UUID, in ProfileInput, meta audit.Meta) (User, error) {
	cur, err := s.Get(ctx, id)
	if err != nil {
		return User{}, err
	}
	name, email := cur.DisplayName, pgtype.Text{}
	if cur.Email != nil {
		email = pgtype.Text{String: *cur.Email, Valid: true}
	}
	var changed []string
	if in.DisplayName != nil {
		if name, err = checkDisplayName(*in.DisplayName); err != nil {
			return User{}, err
		}
		if name != cur.DisplayName {
			changed = append(changed, "display_name")
		}
	}
	if in.Email != nil {
		if email, err = checkEmail(*in.Email); err != nil {
			return User{}, err
		}
		if email.String != deref(cur.Email) {
			changed = append(changed, "email")
		}
	}
	if len(changed) == 0 {
		return cur, nil
	}
	if _, err := s.q.UpdateUserProfile(ctx, repo.UpdateUserProfileParams{ID: id, DisplayName: name, Email: email}); err != nil {
		return User{}, fmt.Errorf("update user: %w", err)
	}
	s.record(ctx, ActionUpdate, audit.ResultSuccess, id.String(), meta, map[string]any{"fields": changed})
	return s.Get(ctx, id)
}

// SetRoles replaces the roles of a user. Removing one's own Admin role, or the Admin role of the last active Admin,
// is refused.
func (s *Service) SetRoles(ctx context.Context, id uuid.UUID, roles []string, meta audit.Meta) (User, error) {
	var u User
	var added, removed []string
	err := s.tx(ctx, func(q *repo.Queries) error {
		roleIDs, err := resolveRoles(ctx, q, roles)
		if err != nil {
			return err
		}
		if _, err := q.LockAdminRole(ctx); err != nil {
			return fmt.Errorf("lock admin role: %w", err)
		}
		cur, err := s.get(ctx, q, id)
		if err != nil {
			return err
		}
		added, removed = diff(cur.Roles, roles)
		if len(added) == 0 && len(removed) == 0 {
			u = cur
			return nil
		}
		if slices.Contains(removed, rbac.RoleAdmin) {
			if err := s.guardAdminLoss(ctx, q, cur); err != nil {
				return err
			}
		}
		if err := q.DeleteUserRoles(ctx, id); err != nil {
			return fmt.Errorf("delete roles: %w", err)
		}
		for _, rid := range roleIDs {
			if err := q.AddUserRole(ctx, repo.AddUserRoleParams{UserID: id, RoleID: rid, GrantedBy: actorID(ctx)}); err != nil {
				return fmt.Errorf("add role: %w", err)
			}
		}
		u, err = s.get(ctx, q, id)
		return err
	})
	if err != nil {
		s.recordRefusal(ctx, ActionRolesUpdate, id, meta, err)
		return User{}, err
	}
	if len(added)+len(removed) > 0 {
		s.record(ctx, ActionRolesUpdate, audit.ResultSuccess, id.String(), meta, map[string]any{"added": added, "removed": removed})
	}
	return u, nil
}

// SetStatus disables (revoking every session) or re-enables an account. Disabling oneself or the last active
// Admin is refused.
func (s *Service) SetStatus(ctx context.Context, id uuid.UUID, status string, meta audit.Meta) (User, error) {
	if status != StatusActive && status != StatusDisabled {
		return User{}, &ValidationError{Field: "status", Reason: "must be active or disabled"}
	}
	action := ActionEnable
	if status == StatusDisabled {
		action = ActionDisable
	}
	var u User
	var revoked int64
	changed := false
	err := s.tx(ctx, func(q *repo.Queries) error {
		if _, err := q.LockAdminRole(ctx); err != nil {
			return fmt.Errorf("lock admin role: %w", err)
		}
		cur, err := s.get(ctx, q, id)
		if err != nil {
			return err
		}
		if cur.Status == status {
			u = cur
			return nil
		}
		if status == StatusDisabled {
			if err := s.guardAdminLoss(ctx, q, cur); err != nil {
				return err
			}
		}
		if _, err := q.SetUserStatus(ctx, repo.SetUserStatusParams{ID: id, Status: status}); err != nil {
			return fmt.Errorf("set status: %w", err)
		}
		if status == StatusDisabled {
			if revoked, err = q.RevokeUserSessions(ctx, id); err != nil {
				return fmt.Errorf("revoke sessions: %w", err)
			}
		}
		changed = true
		u, err = s.get(ctx, q, id)
		return err
	})
	if err != nil {
		s.recordRefusal(ctx, action, id, meta, err)
		return User{}, err
	}
	if changed {
		md := map[string]any{}
		if status == StatusDisabled {
			md["sessions_revoked"] = revoked
		}
		s.record(ctx, action, audit.ResultSuccess, id.String(), meta, md)
	}
	return u, nil
}

// ResetPassword sets a generated temporary password (returned once), forces a change at the next login, clears the
// lockout and revokes every session of the user. Not for one's own account (use the password change instead).
func (s *Service) ResetPassword(ctx context.Context, id uuid.UUID, meta audit.Meta) (string, error) {
	if self(ctx, id) {
		s.recordRefusal(ctx, ActionPasswordReset, id, meta, ErrSelf)
		return "", ErrSelf
	}
	temp, hash, err := s.tempPassword()
	if err != nil {
		return "", err
	}
	var revoked int64
	err = s.tx(ctx, func(q *repo.Queries) error {
		n, err := q.ResetPassword(ctx, repo.ResetPasswordParams{ID: id, PasswordHash: hash})
		if err != nil {
			return fmt.Errorf("reset password: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		revoked, err = q.RevokeUserSessions(ctx, id)
		return err
	})
	if err != nil {
		return "", err
	}
	s.record(ctx, ActionPasswordReset, audit.ResultSuccess, id.String(), meta, map[string]any{"sessions_revoked": revoked})
	return temp, nil
}

// ResetMFA removes the MFA enrolment and recovery codes of a user and revokes every session; the user enrols again
// at the next sign-in. Not for one's own account (D-064). Without a principal in ctx (CLI) any account may be reset.
func (s *Service) ResetMFA(ctx context.Context, id uuid.UUID, meta audit.Meta) error {
	if self(ctx, id) {
		s.recordRefusal(ctx, ActionMFAReset, id, meta, ErrSelf)
		return ErrSelf
	}
	var revoked int64
	err := s.tx(ctx, func(q *repo.Queries) error {
		if _, err := s.get(ctx, q, id); err != nil {
			return err
		}
		if _, err := q.DeleteUserMFA(ctx, id); err != nil {
			return fmt.Errorf("delete mfa: %w", err)
		}
		if err := q.DeleteUserRecoveryCodes(ctx, id); err != nil {
			return fmt.Errorf("delete recovery codes: %w", err)
		}
		var err error
		revoked, err = q.RevokeUserSessions(ctx, id)
		return err
	})
	if err != nil {
		return err
	}
	s.record(ctx, ActionMFAReset, audit.ResultSuccess, id.String(), meta, map[string]any{"sessions_revoked": revoked})
	return nil
}

// Unlock clears a lockout (wrong passwords/MFA codes) before it expires by itself (CLI emergency path; a lockout
// otherwise always ends after AUTH_LOCK_DURATION).
func (s *Service) Unlock(ctx context.Context, id uuid.UUID, meta audit.Meta) error {
	n, err := s.q.UnlockUser(ctx, id)
	if err != nil {
		return fmt.Errorf("unlock: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	s.record(ctx, ActionUnlock, audit.ResultSuccess, id.String(), meta, nil)
	return nil
}

// UserIDByUsername resolves a username (CLI).
func (s *Service) UserIDByUsername(ctx context.Context, username string) (uuid.UUID, error) {
	id, err := s.q.GetUserIDByUsername(ctx, strings.TrimSpace(username))
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

// guardAdminLoss refuses changes that take away cur's Admin access (role removal or disabling) when cur is the
// actor, or when no other active Admin would remain. Call with the admin role row locked.
func (s *Service) guardAdminLoss(ctx context.Context, q *repo.Queries, cur User) error {
	if self(ctx, cur.ID) {
		return ErrSelf
	}
	if !slices.Contains(cur.Roles, rbac.RoleAdmin) || cur.Status != StatusActive {
		return nil
	}
	n, err := q.CountOtherActiveAdmins(ctx, cur.ID)
	if err != nil {
		return fmt.Errorf("count admins: %w", err)
	}
	if n == 0 {
		return ErrLastAdmin
	}
	return nil
}

func (s *Service) get(ctx context.Context, q *repo.Queries, id uuid.UUID) (User, error) {
	r, err := q.GetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("get user: %w", err)
	}
	return view(r), nil
}

func (s *Service) tx(ctx context.Context, fn func(q *repo.Queries) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) tempPassword() (string, string, error) {
	temp, err := TemporaryPassword()
	if err != nil {
		return "", "", err
	}
	hash, err := password.Hash(temp, s.params)
	if err != nil {
		return "", "", err
	}
	return temp, hash, nil
}

func (s *Service) record(ctx context.Context, action, result, target string, meta audit.Meta, md map[string]any) {
	ev := audit.Event{Action: action, Result: result, TargetType: "user", TargetID: target, Meta: meta, Metadata: md}
	if _, ok := principal.From(ctx); !ok {
		ev.ActorType, ev.ActorUsername = audit.ActorSystem, "cli" // server-side command (no session)
	}
	_ = s.audit.Record(ctx, ev)
}

// recordRefusal audits refusals that matter for security (self-actions, last Admin); validation errors are not
// audited.
func (s *Service) recordRefusal(ctx context.Context, action string, id uuid.UUID, meta audit.Meta, err error) {
	reason := ""
	switch {
	case errors.Is(err, ErrSelf):
		reason = "self"
	case errors.Is(err, ErrLastAdmin):
		reason = "last_admin"
	default:
		return
	}
	s.record(ctx, action, audit.ResultDenied, id.String(), meta, map[string]any{"reason": reason})
}

// tempAlphabet avoids look-alike characters (0/O, 1/l/I) so the password can be read out or typed from paper.
const tempAlphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// TemporaryPassword returns 16 random characters in four groups (xxxx-xxxx-xxxx-xxxx, ≈ 92 bits).
func TemporaryPassword() (string, error) {
	var b strings.Builder
	size := big.NewInt(int64(len(tempAlphabet)))
	for i := range 16 {
		if i > 0 && i%4 == 0 {
			b.WriteByte('-')
		}
		n, err := rand.Int(rand.Reader, size)
		if err != nil {
			return "", err
		}
		b.WriteByte(tempAlphabet[n.Int64()])
	}
	return b.String(), nil
}

func resolveRoles(ctx context.Context, q *repo.Queries, keys []string) ([]uuid.UUID, error) {
	if len(keys) == 0 {
		return nil, &ValidationError{Field: "roles", Reason: "at least one role is required"}
	}
	if len(keys) > MaxRoles {
		return nil, &ValidationError{Field: "roles", Reason: "too many roles"}
	}
	uniq := slices.Clone(keys)
	slices.Sort(uniq)
	if len(slices.Compact(uniq)) != len(keys) {
		return nil, &ValidationError{Field: "roles", Reason: "duplicate role"}
	}
	rows, err := q.GetRolesByKeys(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("get roles: %w", err)
	}
	if len(rows) != len(keys) {
		return nil, &ValidationError{Field: "roles", Reason: "unknown role"}
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return ids, nil
}

func checkDisplayName(v string) (string, error) {
	v = strings.TrimSpace(v)
	if n := utf8.RuneCountInString(v); n < 1 || n > MaxDisplayName || strings.ContainsAny(v, "\x00\r\n\t") {
		return "", &ValidationError{Field: "display_name", Reason: "1–128 characters, single line"}
	}
	return v, nil
}

func checkEmail(v string) (pgtype.Text, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return pgtype.Text{}, nil
	}
	if len(v) > MaxEmail || !emailRE.MatchString(v) {
		return pgtype.Text{}, &ValidationError{Field: "email", Reason: "invalid e-mail address"}
	}
	return pgtype.Text{String: v, Valid: true}, nil
}

func view(r repo.GetUserRow) User {
	u := User{ID: r.ID, Username: r.Username, DisplayName: r.DisplayName, Status: r.Status, Roles: r.Roles,
		MustChangePassword: r.MustChangePassword, Locked: r.Locked, MFAEnabled: r.MfaEnabled, CreatedAt: r.CreatedAt.Time,
		UpdatedAt: r.UpdatedAt.Time}
	if u.Roles == nil {
		u.Roles = []string{}
	}
	if r.Email != "" {
		e := r.Email
		u.Email = &e
	}
	if r.LastLoginAt.Valid {
		t := r.LastLoginAt.Time
		u.LastLoginAt = &t
	}
	return u
}

// diff returns the roles in next but not in cur (added) and in cur but not in next (removed), sorted.
func diff(cur, next []string) (added, removed []string) {
	added, removed = []string{}, []string{}
	for _, r := range next {
		if !slices.Contains(cur, r) {
			added = append(added, r)
		}
	}
	for _, r := range cur {
		if !slices.Contains(next, r) {
			removed = append(removed, r)
		}
	}
	slices.Sort(added)
	slices.Sort(removed)
	return added, removed
}

func actorID(ctx context.Context) *uuid.UUID {
	if p, ok := principal.From(ctx); ok {
		id := p.ID
		return &id
	}
	return nil
}

func self(ctx context.Context, id uuid.UUID) bool {
	p, ok := principal.From(ctx)
	return ok && p.ID == id
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// likeEscape escapes LIKE wildcards so the search is literal (the query uses ESCAPE '\').
func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
