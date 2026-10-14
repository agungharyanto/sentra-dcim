package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	audit "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/auth/mfa"
	"github.com/agungharyanto/sentra-dcim/internal/auth/repo"
)

// Enrolment is what the user scans or types into an authenticator app. Sent once to the user's own pending
// session; never logged or audited.
type Enrolment struct {
	Issuer  string   `json:"issuer"`
	Account string   `json:"account"`
	Secret  string   `json:"secret"`      // base32, grouped by four for manual entry
	URI     string   `json:"otpauth_uri"` // otpauth://totp/…
	QR      []string `json:"qr"`          // QR modules, rows of '1'/'0'
}

// MFAStatus is the account security summary.
type MFAStatus struct {
	Enabled           bool    `json:"enabled"`
	EnabledAt         *string `json:"enabled_at"`
	RecoveryRemaining int64   `json:"recovery_codes_remaining"`
}

func (s *Service) mfaEnabled(ctx context.Context, userID uuid.UUID) (bool, error) {
	m, err := s.q.GetUserMFA(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get mfa: %w", err)
	}
	return m.EnabledAt.Valid, nil
}

// MFAStatus returns whether MFA is active and how many recovery codes are left.
func (s *Service) MFAStatus(ctx context.Context, sess Session) (MFAStatus, error) {
	m, err := s.q.GetUserMFA(ctx, sess.User.ID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !m.EnabledAt.Valid) {
		return MFAStatus{}, nil
	}
	if err != nil {
		return MFAStatus{}, fmt.Errorf("get mfa: %w", err)
	}
	n, err := s.q.CountUnusedRecoveryCodes(ctx, sess.User.ID)
	if err != nil {
		return MFAStatus{}, fmt.Errorf("count recovery codes: %w", err)
	}
	at := m.EnabledAt.Time.UTC().Format("2006-01-02T15:04:05Z")
	return MFAStatus{Enabled: true, EnabledAt: &at, RecoveryRemaining: n}, nil
}

// StartEnrolment creates (or replaces) a pending secret for an account without active MFA.
func (s *Service) StartEnrolment(ctx context.Context, sess Session, meta audit.Meta) (Enrolment, error) {
	secret, err := mfa.NewSecret()
	if err != nil {
		return Enrolment{}, err
	}
	sealed, err := s.mfa.Encrypt(secret, sess.User.ID[:])
	if err != nil {
		return Enrolment{}, err
	}
	if _, err := s.q.UpsertPendingMFA(ctx, repo.UpsertPendingMFAParams{UserID: sess.User.ID, SecretEnc: sealed}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Enrolment{}, ErrMFAEnabled
		}
		return Enrolment{}, fmt.Errorf("store pending mfa: %w", err)
	}
	s.record(ctx, sess, ActionMFAEnrolStart, audit.ResultSuccess, meta, nil) // never the secret
	uri := mfa.URI(s.cfg.MFAIssuer, sess.User.Username, secret)
	qr, err := mfa.QR(uri)
	if err != nil {
		return Enrolment{}, err
	}
	return Enrolment{Issuer: s.cfg.MFAIssuer, Account: sess.User.Username, Secret: mfa.GroupedSecret(secret), URI: uri, QR: qr}, nil
}

// ConfirmEnrolment activates MFA with the first valid code, completes the sign-in of this session and returns the
// recovery codes (shown once).
func (s *Service) ConfirmEnrolment(ctx context.Context, sess Session, code string, meta audit.Meta) ([]string, error) {
	if err := s.mfaPrecheck(ctx, sess, meta); err != nil {
		return nil, err
	}
	m, err := s.q.GetUserMFA(ctx, sess.User.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMFANotEnrolled
	}
	if err != nil {
		return nil, fmt.Errorf("get mfa: %w", err)
	}
	if m.EnabledAt.Valid {
		return nil, ErrMFAEnabled
	}
	secret, err := s.mfa.Decrypt(m.SecretEnc, sess.User.ID[:])
	if err != nil {
		return nil, err
	}
	step, ok := mfa.Verify(secret, code, s.now(), m.LastUsedStep)
	if !ok {
		return nil, s.mfaFailed(ctx, sess, ActionMFAEnrol, meta)
	}
	if n, err := s.q.EnableMFA(ctx, repo.EnableMFAParams{UserID: sess.User.ID, Step: step}); err != nil || n == 0 {
		if err != nil {
			return nil, fmt.Errorf("enable mfa: %w", err)
		}
		return nil, s.mfaFailed(ctx, sess, ActionMFAEnrol, meta)
	}
	codes, err := s.replaceRecoveryCodes(ctx, sess.User.ID)
	if err != nil {
		return nil, err
	}
	s.record(ctx, sess, ActionMFAEnrol, audit.ResultSuccess, meta, nil)
	return codes, s.completeMFA(ctx, sess, "enrol", meta)
}

// VerifyMFA completes the sign-in with a TOTP code or a one-time recovery code.
func (s *Service) VerifyMFA(ctx context.Context, sess Session, code string, meta audit.Meta) error {
	if sess.MFAVerified {
		return nil
	}
	if err := s.mfaPrecheck(ctx, sess, meta); err != nil {
		return err
	}
	m, err := s.q.GetUserMFA(ctx, sess.User.ID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !m.EnabledAt.Valid) {
		return ErrMFANotEnrolled
	}
	if err != nil {
		return fmt.Errorf("get mfa: %w", err)
	}
	if mfa.LooksLikeRecovery(code) {
		n, err := s.q.UseRecoveryCode(ctx, repo.UseRecoveryCodeParams{UserID: sess.User.ID, CodeHash: s.mfa.HashRecovery(code)})
		if err != nil {
			return fmt.Errorf("use recovery code: %w", err)
		}
		if n == 0 {
			return s.mfaFailed(ctx, sess, ActionMFAVerify, meta)
		}
		left, _ := s.q.CountUnusedRecoveryCodes(ctx, sess.User.ID)
		s.record(ctx, sess, ActionMFAVerify, audit.ResultSuccess, meta, map[string]any{"method": "recovery_code", "recovery_codes_remaining": left})
		return s.completeMFA(ctx, sess, "recovery_code", meta)
	}
	secret, err := s.mfa.Decrypt(m.SecretEnc, sess.User.ID[:])
	if err != nil {
		return err
	}
	step, ok := mfa.Verify(secret, code, s.now(), m.LastUsedStep)
	if !ok {
		return s.mfaFailed(ctx, sess, ActionMFAVerify, meta)
	}
	if n, err := s.q.UseTOTPStep(ctx, repo.UseTOTPStepParams{UserID: sess.User.ID, Step: step}); err != nil || n == 0 {
		if err != nil {
			return fmt.Errorf("use totp step: %w", err)
		}
		return s.mfaFailed(ctx, sess, ActionMFAVerify, meta) // replay raced in
	}
	s.record(ctx, sess, ActionMFAVerify, audit.ResultSuccess, meta, map[string]any{"method": "totp"})
	return s.completeMFA(ctx, sess, "totp", meta)
}

// RegenerateRecoveryCodes replaces all recovery codes; requires a current TOTP code (re-authentication).
func (s *Service) RegenerateRecoveryCodes(ctx context.Context, sess Session, code string, meta audit.Meta) ([]string, error) {
	if err := s.mfaPrecheck(ctx, sess, meta); err != nil {
		return nil, err
	}
	m, err := s.q.GetUserMFA(ctx, sess.User.ID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !m.EnabledAt.Valid) {
		return nil, ErrMFANotEnrolled
	}
	if err != nil {
		return nil, fmt.Errorf("get mfa: %w", err)
	}
	secret, err := s.mfa.Decrypt(m.SecretEnc, sess.User.ID[:])
	if err != nil {
		return nil, err
	}
	step, ok := mfa.Verify(secret, code, s.now(), m.LastUsedStep)
	if !ok {
		return nil, s.mfaFailed(ctx, sess, ActionMFARecovery, meta)
	}
	if n, err := s.q.UseTOTPStep(ctx, repo.UseTOTPStepParams{UserID: sess.User.ID, Step: step}); err != nil || n == 0 {
		if err != nil {
			return nil, fmt.Errorf("use totp step: %w", err)
		}
		return nil, s.mfaFailed(ctx, sess, ActionMFARecovery, meta)
	}
	codes, err := s.replaceRecoveryCodes(ctx, sess.User.ID)
	if err != nil {
		return nil, err
	}
	s.record(ctx, sess, ActionMFARecovery, audit.ResultSuccess, meta, map[string]any{"count": len(codes)})
	return codes, nil
}

// mfaPrecheck applies the login rate limits (separate counters for MFA codes) and refuses locked accounts.
func (s *Service) mfaPrecheck(ctx context.Context, sess Session, meta audit.Meta) error {
	if err := s.rateLimitKeys(ctx, "mfa:", sess.User.Username, meta); err != nil {
		if errors.Is(err, ErrRateLimited) {
			s.record(ctx, sess, ActionMFAVerify, audit.ResultDenied, meta, map[string]any{"reason": "rate_limited"})
		}
		return err
	}
	u, err := s.q.GetUserByID(ctx, sess.User.ID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if u.LockedUntil.Valid && u.LockedUntil.Time.After(s.now()) {
		s.record(ctx, sess, ActionMFAVerify, audit.ResultDenied, meta, map[string]any{"reason": "locked"})
		_, _ = s.q.RevokeUserSessions(ctx, sess.User.ID)
		return ErrInvalidMFACode
	}
	return nil
}

// mfaFailed counts a wrong code towards the account lockout shared with passwords; reaching it revokes every
// session of the account (the pending one included).
func (s *Service) mfaFailed(ctx context.Context, sess Session, action string, meta audit.Meta) error {
	row, err := s.q.RecordLoginFailure(ctx, repo.RecordLoginFailureParams{
		UserID: sess.User.ID, Threshold: int32(s.cfg.LockThreshold), LockSeconds: s.cfg.LockDuration.Seconds(), //nolint:gosec // bounded in config
	})
	if err != nil {
		return fmt.Errorf("record failure: %w", err)
	}
	s.record(ctx, sess, action, audit.ResultFailure, meta, map[string]any{"reason": "invalid_code", "failed_attempts": row.FailedAttempts})
	if int(row.FailedAttempts) >= s.cfg.LockThreshold && row.LockedUntil.Valid {
		id := sess.User.ID
		_ = s.audit.Record(ctx, audit.Event{Action: ActionLockout, Result: audit.ResultDenied, ActorUsername: sess.User.Username,
			ActorID: &id, TargetType: "user", TargetID: id.String(), Meta: meta,
			Metadata: map[string]any{"failed_attempts": row.FailedAttempts, "locked_until": row.LockedUntil.Time.UTC(), "stage": "mfa"}})
		_, _ = s.q.RevokeUserSessions(ctx, sess.User.ID)
	}
	return ErrInvalidMFACode
}

// completeMFA marks the session verified and records the completed sign-in (resets the failure counter).
func (s *Service) completeMFA(ctx context.Context, sess Session, method string, meta audit.Meta) error {
	if err := s.q.MarkSessionMFAVerified(ctx, sess.Info.ID); err != nil {
		return fmt.Errorf("mark session: %w", err)
	}
	if err := s.q.RecordLoginSuccess(ctx, sess.User.ID); err != nil {
		return fmt.Errorf("record success: %w", err)
	}
	s.record(ctx, sess, ActionLoginSuccess, audit.ResultSuccess, meta, map[string]any{"mfa": method})
	return nil
}

func (s *Service) replaceRecoveryCodes(ctx context.Context, userID uuid.UUID) ([]string, error) {
	codes, err := mfa.NewRecoveryCodes()
	if err != nil {
		return nil, err
	}
	if err := s.q.DeleteRecoveryCodes(ctx, userID); err != nil {
		return nil, fmt.Errorf("delete recovery codes: %w", err)
	}
	for _, c := range codes {
		if err := s.q.InsertRecoveryCode(ctx, repo.InsertRecoveryCodeParams{UserID: userID, CodeHash: s.mfa.HashRecovery(c)}); err != nil {
			return nil, fmt.Errorf("store recovery code: %w", err)
		}
	}
	return codes, nil
}

func (s *Service) record(ctx context.Context, sess Session, action, result string, meta audit.Meta, md map[string]any) {
	id := sess.User.ID
	_ = s.audit.Record(ctx, audit.Event{Action: action, Result: result, ActorType: audit.ActorUser, ActorID: &id,
		ActorUsername: sess.User.Username, TargetType: "session", TargetID: sess.Info.ID.String(), Meta: meta, Metadata: md})
}
