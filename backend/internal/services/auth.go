package services

import (
	"context"
	"encoding/base32"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/database"
	"opensex/backend/internal/models"
	"opensex/backend/internal/rbac"
	"opensex/backend/internal/validators"
	"opensex/backend/pkg/crypto"
	"opensex/backend/pkg/jwt"
	"opensex/backend/pkg/ratelimit"
)

type AuthService struct{ *Services }

const (
	recoveryCodeCount   = 10
	refreshReuseGrace   = 10 * time.Second
	lockoutThreshold    = 10
	recoverFailureLimit = 10
	recoverBlock        = 24 * time.Hour
	tempPasswordLength  = 20
)

var lockoutSteps = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour}

// Rate-limit policies (§13.3) applied inside the services on the targeted
// identifier; per-IP limits are applied by the middleware.
var (
	LimitLoginUser      = []ratelimit.Limit{ratelimit.PerMinute(5), ratelimit.PerHour(20)}
	LimitAdminLoginUser = []ratelimit.Limit{ratelimit.PerMinute(3), ratelimit.PerHour(10)}
	LimitRecoverUser    = []ratelimit.Limit{ratelimit.PerHour(5)}
	LimitPasswordChange = []ratelimit.Limit{ratelimit.PerHour(10)}
)

var errInvalidCredentials = apperr.New(apperr.KindUnauthorized, "invalid_credentials", "invalid username or password")

type Session struct {
	AccessToken      string    `json:"access_token"`
	TokenType        string    `json:"token_type"`
	ExpiresAt        time.Time `json:"expires_at"`
	RefreshToken     string    `json:"-"`
	RefreshExpiresAt time.Time `json:"-"`
	SessionID        uuid.UUID `json:"-"`
}

type AuthResult struct {
	Session       *Session
	User          *models.User
	RecoveryCodes []string
}

// ---------------------------------------------------------------- registration

type RegisterInput struct {
	Username          string
	Password          string
	EstablishmentSlug string
	DisplayName       string
}

func (s *AuthService) checkCredentials(ctx context.Context, in RegisterInput, admin bool) error {
	var c apperr.Collector
	if code := validators.Username(in.Username); code != "" {
		c.Add("username", code)
	}
	if code := validators.PasswordLength(in.Password, admin); code != "" {
		c.Add("password", code)
	} else if strings.EqualFold(in.Password, in.Username) {
		c.Add("password", "password_equals_username")
	} else if s.Breach.IsBreached(ctx, in.Password) {
		c.Add("password", "password_breached")
	}
	return c.Err()
}

func (s *AuthService) newUser(ctx context.Context, in RegisterInput) (*models.User, error) {
	hash, err := s.Hasher.Hash(ctx, in.Password)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	now := s.now()
	return &models.User{
		ID: models.NewID(), Username: in.Username, PasswordHash: hash, Status: models.UserActive,
		TermsVersionAccepted: s.Cfg.TermsVersion, TermsAcceptedAt: now, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (s *AuthService) createUser(ctx context.Context, u *models.User, role string) error {
	if err := s.Repos.Users.Create(ctx, u, role); err != nil {
		if database.IsUniqueViolation(err) {
			return apperr.Conflict("username_taken", "username already taken")
		}
		return apperr.Internal(err)
	}
	return nil
}

// RegisterClient creates a CLIENT account (role fixed by the endpoint, RM-01).
func (s *AuthService) RegisterManager(ctx context.Context, in RegisterInput) (*AuthResult, error) {
	err := s.checkCredentials(ctx, in, false)
	var c apperr.Collector
	if err != nil || c.Err() != nil {
		return nil, mergeValidation(err, c.Err())
	}
	u, err := s.newUser(ctx, in)
	if err != nil {
		return nil, err
	}
	var codes []string
	err = s.tx(ctx, func(ctx context.Context) error {
		if err := s.createUser(ctx, u, models.RoleManager); err != nil {
			return err
		}
		now := s.now()
		e := &models.Establishment{
			ID: models.NewID(), ManagerUserID: u.ID, Slug: "etablissement-" + strings.ToLower(crypto.RandomPassword(8)),
			OpeningHours: models.JSON("{}"), CreatedAt: now, UpdatedAt: now,
		}
		if err := s.Repos.Establishments.Create(ctx, e); err != nil {
			return apperr.Internal(err)
		}
		codes, err = s.issueRecoveryCodes(ctx, u.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.afterRegister(ctx, u, codes)
}

func (s *AuthService) afterRegister(ctx context.Context, u *models.User, codes []string) (*AuthResult, error) {
	sess, err := s.createSession(ctx, u, nil)
	if err != nil {
		return nil, err
	}
	return &AuthResult{Session: sess, User: u, RecoveryCodes: codes}, nil
}

func mergeValidation(errs ...error) error {
	var fields []apperr.FieldError
	for _, err := range errs {
		if e, ok := apperr.As(err); ok {
			fields = append(fields, e.Fields...)
		} else if err != nil {
			return err
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return apperr.Validation(fields...)
}

// ---------------------------------------------------------------- recovery codes

var codeEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func normalizeRecoveryCode(c string) string {
	c = strings.ToUpper(c)
	c = strings.NewReplacer("-", "", " ", "").Replace(c)
	return c
}

func (s *AuthService) hashRecoveryCode(code string) []byte {
	return crypto.HMACSHA256(s.Cfg.RecoveryCodeKey, []byte(normalizeRecoveryCode(code)))
}

// issueRecoveryCodes replaces the codes of a user with 10 new 80-bit codes
// and returns them in clear text (shown once, RM-04).
func (s *AuthService) issueRecoveryCodes(ctx context.Context, userID uuid.UUID) ([]string, error) {
	now := s.now()
	codes := make([]string, recoveryCodeCount)
	rows := make([]models.RecoveryCode, recoveryCodeCount)
	for i := range codes {
		raw := codeEncoding.EncodeToString(crypto.RandomBytes(10)) // 16 chars, 80 bits
		codes[i] = raw[0:4] + "-" + raw[4:8] + "-" + raw[8:12] + "-" + raw[12:16]
		rows[i] = models.RecoveryCode{ID: models.NewID(), UserID: userID, CodeHash: s.hashRecoveryCode(raw), CreatedAt: now}
	}
	if err := s.Repos.Tokens.ReplaceRecoveryCodes(ctx, userID, rows); err != nil {
		return nil, apperr.Internal(err)
	}
	if err := s.Repos.Users.Update(ctx, userID, map[string]any{"recovery_codes_confirmed_at": nil}); err != nil {
		return nil, apperr.Internal(err)
	}
	return codes, nil
}

func (s *AuthService) ConfirmRecoveryCodes(ctx context.Context, actor *rbac.Actor) error {
	if err := s.Repos.Users.Update(ctx, actor.UserID, map[string]any{"recovery_codes_confirmed_at": s.now()}); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

type RecoveryStatus struct {
	Remaining int64 `json:"remaining"`
	Confirmed bool  `json:"confirmed"`
	Low       bool  `json:"low"`
}

func (s *AuthService) RecoveryCodesStatus(ctx context.Context, actor *rbac.Actor) (*RecoveryStatus, error) {
	u, err := s.Repos.Users.ByID(ctx, actor.UserID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	n, err := s.Repos.Tokens.RemainingRecoveryCodes(ctx, actor.UserID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &RecoveryStatus{Remaining: n, Confirmed: u.RecoveryCodesConfirmedAt != nil, Low: n <= 3}, nil
}

func (s *AuthService) RegenerateRecoveryCodes(ctx context.Context, actor *rbac.Actor, pwd string) ([]string, error) {
	u, err := s.Repos.Users.ByID(ctx, actor.UserID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	if ok, _, _ := s.Hasher.Verify(ctx, pwd, u.PasswordHash); !ok {
		return nil, apperr.New(apperr.KindForbidden, "invalid_password", "password is incorrect")
	}
	var codes []string
	err = s.tx(ctx, func(ctx context.Context) error {
		var err error
		codes, err = s.issueRecoveryCodes(ctx, u.ID)
		return err
	})
	return codes, err
}

// Recover resets a password with a recovery code (§3.5). Every failure
// returns the same generic error.
func (s *AuthService) Recover(ctx context.Context, username, code, newPassword string) error {
	generic := apperr.New(apperr.KindUnauthorized, "invalid_recovery", "invalid username or recovery code")
	key := "recover:fail:" + strings.ToLower(username)
	if res, err := s.Limiter.Allow(ctx, "recover:user:"+strings.ToLower(username), LimitRecoverUser...); err == nil && !res.Allowed {
		return apperr.RateLimited(res.RetryAfter)
	}
	fails, _ := s.Redis.Get(ctx, key).Int()
	if fails >= recoverFailureLimit {
		return apperr.RateLimited(recoverBlock)
	}
	if code := validators.PasswordLength(newPassword, false); code != "" {
		return apperr.Invalid("new_password", code)
	}
	fail := func(reason string) error {
		s.Redis.Incr(ctx, key)
		s.Redis.Expire(ctx, key, recoverBlock)
		return generic
	}
	u, err := s.Repos.Users.ByUsername(ctx, username)
	if err != nil {
		if database.IsNotFound(err) {
			return fail("unknown_user")
		}
		return apperr.Internal(err)
	}
	if u.Status != models.UserActive {
		return fail("inactive")
	}
	admin := u.Role == models.RoleAdmin
	if c := validators.PasswordLength(newPassword, admin); c != "" {
		return apperr.Invalid("new_password", c)
	}
	if s.Breach.IsBreached(ctx, newPassword) {
		return apperr.Invalid("new_password", "password_breached")
	}
	hash, err := s.Hasher.Hash(ctx, newPassword)
	if err != nil {
		return apperr.Internal(err)
	}
	matched := false
	var families []uuid.UUID
	err = s.tx(ctx, func(ctx context.Context) error {
		ok, err := s.Repos.Tokens.ConsumeRecoveryCode(ctx, u.ID, s.hashRecoveryCode(code), s.now())
		if err != nil || !ok {
			return err
		}
		matched = true
		if err := s.Repos.Users.Update(ctx, u.ID, map[string]any{"password_hash": hash, "must_change_password": false,
			"failed_login_count": 0, "locked_until": nil}); err != nil {
			return err
		}
		families, err = s.Repos.Tokens.RevokeUser(ctx, u.ID, nil, models.RevokePasswordRecovered, s.now())
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return apperr.Internal(err)
	}
	if !matched {
		return fail("bad_code")
	}
	s.Redis.Del(ctx, key)
	_ = s.Denylist.DenySessions(ctx, families...)
	return nil
}

// ---------------------------------------------------------------- login & sessions

type LoginInput struct {
	Username string
	Password string
}

// Login authenticates a non-admin (admin=false) or an admin (admin=true)
// account. Admin accounts can only log in through the admin endpoint (§13.6).
func (s *AuthService) Login(ctx context.Context, in LoginInput, admin bool) (*AuthResult, error) {
	limits := LimitLoginUser
	if admin {
		limits = LimitAdminLoginUser
	}
	userKey := "login:user:" + strings.ToLower(in.Username)
	res, err := s.Limiter.Allow(ctx, userKey, limits...)
	if err != nil && admin {
		return nil, apperr.Unavailable("rate_limit_unavailable", err)
	}
	if err == nil && !res.Allowed {
		return nil, apperr.RateLimited(res.RetryAfter)
	}
	failed := func(u *models.User, reason string) error {
		if admin {
			targetID := ""
			if u != nil {
				targetID = u.ID.String()
			}
			s.Alert.Send(ctx, "auth.login.failed", models.SeverityWarn, "user", targetID)
		}
		return errInvalidCredentials
	}
	if len(in.Password) > 128*4 || in.Username == "" {
		return nil, failed(nil, "malformed")
	}
	u, err := s.Repos.Users.ByUsername(ctx, in.Username)
	if err != nil {
		if database.IsNotFound(err) {
			s.Hasher.VerifyDummy(ctx, in.Password)
			return nil, failed(nil, "unknown_user")
		}
		return nil, apperr.Internal(err)
	}
	now := s.now()
	if u.LockedUntil != nil && u.LockedUntil.After(now) {
		return nil, apperr.RateLimited(u.LockedUntil.Sub(now))
	}
	ok, rehash, err := s.Hasher.Verify(ctx, in.Password, u.PasswordHash)
	if err != nil && !errors.Is(err, context.Canceled) {
		return nil, apperr.Internal(err)
	}
	if !ok {
		count := u.FailedLoginCount + 1
		cols := map[string]any{"failed_login_count": count}
		if count >= lockoutThreshold {
			step := count - lockoutThreshold
			if step >= len(lockoutSteps) {
				step = len(lockoutSteps) - 1
			}
			cols["locked_until"] = now.Add(lockoutSteps[step])
		}
		_ = s.Repos.Users.Update(ctx, u.ID, cols)
		return nil, failed(u, "bad_password")
	}
	if u.Status != models.UserActive {
		return nil, failed(u, "status_"+strings.ToLower(u.Status))
	}
	if (u.Role == models.RoleAdmin) != admin {
		return nil, failed(u, "wrong_endpoint")
	}
	cols := map[string]any{"failed_login_count": 0, "locked_until": nil, "last_login_at": now}
	if rehash {
		if h, err := s.Hasher.Hash(ctx, in.Password); err == nil {
			cols["password_hash"] = h
		}
	}
	if err := s.Repos.Users.Update(ctx, u.ID, cols); err != nil {
		return nil, apperr.Internal(err)
	}
	_ = s.Limiter.Reset(ctx, userKey, limits...)
	if admin {
		s.alertNewAdminDevice(ctx, u)
	}
	sess, err := s.createSession(ctx, u, nil)
	if err != nil {
		return nil, err
	}
	return &AuthResult{Session: sess, User: u}, nil
}

func (s *AuthService) alertNewAdminDevice(ctx context.Context, u *models.User) {
	meta := rbac.MetaFrom(ctx)
	var n int64
	s.DB.Conn(ctx).Model(&models.RefreshToken{}).
		Where("user_id = ? AND ip_hash = ? AND user_agent = ?", u.ID, meta.IPHash, truncate(meta.UserAgent, 255)).Count(&n)
	if n == 0 {
		s.Alert.Send(ctx, "auth.admin_login.new_device", models.SeverityWarn, "user", u.ID.String())
	}
}

func (s *AuthService) ttls(role string) (access, sliding, absolute time.Duration) {
	if role == models.RoleAdmin {
		return s.Cfg.AdminAccessTTL, s.Cfg.AdminRefreshTTL, s.Cfg.AdminRefreshAbsTTL
	}
	return s.Cfg.AccessTTL, s.Cfg.RefreshTTL, s.Cfg.RefreshAbsTTL
}

func hashRefresh(raw string) []byte { return crypto.SHA256([]byte(raw)) }

// createSession starts a new refresh family (parent == nil) or rotates it.
func (s *AuthService) createSession(ctx context.Context, u *models.User, parent *models.RefreshToken) (*Session, error) {
	accessTTL, sliding, absolute := s.ttls(u.Role)
	now := s.now()
	meta := rbac.MetaFrom(ctx)
	raw := crypto.RandomToken(32)
	t := &models.RefreshToken{
		ID: models.NewID(), UserID: u.ID, TokenHash: hashRefresh(raw), CreatedAt: now,
		UserAgent: truncate(meta.UserAgent, 255), IPHash: meta.IPHash,
	}
	if parent == nil {
		t.FamilyID = models.NewID()
		t.AbsoluteExpiresAt = now.Add(absolute)
	} else {
		t.FamilyID = parent.FamilyID
		t.ParentID = &parent.ID
		t.AbsoluteExpiresAt = parent.AbsoluteExpiresAt
	}
	t.ExpiresAt = now.Add(sliding)
	if t.ExpiresAt.After(t.AbsoluteExpiresAt) {
		t.ExpiresAt = t.AbsoluteExpiresAt
	}
	if err := s.Repos.Tokens.CreateRefresh(ctx, t); err != nil {
		return nil, apperr.Internal(err)
	}
	access, exp, err := s.JWT.Issue(jwt.IssueParams{Subject: u.ID.String(), Role: u.Role, SID: t.FamilyID.String(), TTL: accessTTL})
	if err != nil {
		return nil, apperr.Internal(err)
	}
	return &Session{AccessToken: access, TokenType: "Bearer", ExpiresAt: exp, RefreshToken: raw, RefreshExpiresAt: t.ExpiresAt, SessionID: t.FamilyID}, nil
}

// Refresh rotates a refresh token (§12.1). Reusing an already rotated token
// outside the 10 s grace window revokes the whole family.
func (s *AuthService) Refresh(ctx context.Context, raw string) (*AuthResult, error) {
	if raw == "" {
		return nil, apperr.Unauthorized("invalid_refresh")
	}
	var (
		result   *AuthResult
		outcome  string
		familyID uuid.UUID
	)
	err := s.tx(ctx, func(ctx context.Context) error {
		t, err := s.Repos.Tokens.RefreshByHashForUpdate(ctx, hashRefresh(raw))
		if err != nil {
			if database.IsNotFound(err) {
				outcome = "unknown"
				return nil
			}
			return err
		}
		familyID = t.FamilyID
		now := s.now()
		if t.RevokedAt != nil {
			if deref(t.RevokedReason) == models.RevokeRotated && now.Sub(*t.RevokedAt) <= refreshReuseGrace {
				outcome = "concurrent"
				return nil
			}
			outcome = "reuse"
			if deref(t.RevokedReason) == models.RevokeRotated {
				return s.Repos.Tokens.RevokeFamily(ctx, t.FamilyID, models.RevokeReuseDetected, now)
			}
			outcome = "revoked"
			return nil
		}
		if !now.Before(t.ExpiresAt) || !now.Before(t.AbsoluteExpiresAt) {
			outcome = "expired"
			return nil
		}
		u, err := s.Repos.Users.ByID(ctx, t.UserID)
		if err != nil {
			if database.IsNotFound(err) {
				outcome = "user_gone"
				return nil
			}
			return err
		}
		if u.Status != models.UserActive {
			outcome = "inactive"
			return s.Repos.Tokens.RevokeFamily(ctx, t.FamilyID, models.RevokeSuspended, now)
		}
		if u.Role == models.RoleProfessional || u.Role == models.RoleManager {
			if active, err := s.employerActive(ctx, u); err != nil {
				return err
			} else if !active {
				outcome = "inactive"
				return nil
			}
		}
		if err := s.Repos.Tokens.RevokeRefresh(ctx, t.ID, models.RevokeRotated, now); err != nil {
			return err
		}
		sess, err := s.createSession(ctx, u, t)
		if err != nil {
			return err
		}
		result = &AuthResult{Session: sess, User: u}
		return nil
	})
	if err != nil {
		return nil, apperr.Internal(err)
	}
	switch outcome {
	case "":
		return result, nil
	case "reuse":
		_ = s.Denylist.DenySessions(ctx, familyID)
	}
	return nil, apperr.Unauthorized("invalid_refresh")
}

// employerActive checks, for professionals, that the establishment's manager
// account is active (RM-63) and, for managers, that the account exists.
func (s *AuthService) employerActive(ctx context.Context, u *models.User) (bool, error) {
	if u.Role != models.RoleProfessional {
		return true, nil
	}
	p, err := s.Repos.Profiles.ByUser(ctx, u.ID)
	if err != nil {
		if database.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	e, err := s.Repos.Establishments.ByID(ctx, p.EstablishmentID)
	if err != nil {
		if database.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	m, err := s.Repos.Users.ByID(ctx, e.ManagerUserID)
	if err != nil {
		if database.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return m.Status == models.UserActive, nil
}

// Logout revokes the session identified by the refresh cookie and/or the
// access token's sid.
func (s *AuthService) Logout(ctx context.Context, rawRefresh string, actor *rbac.Actor) error {
	now := s.now()
	var fams []uuid.UUID
	if rawRefresh != "" {
		if t, err := s.Repos.Tokens.RefreshByHash(ctx, hashRefresh(rawRefresh)); err == nil {
			if actor == nil || actor.UserID == t.UserID {
				fams = append(fams, t.FamilyID)
			}
		}
	}
	if actor != nil {
		fams = append(fams, actor.SessionID)
	}
	for _, f := range fams {
		if err := s.Repos.Tokens.RevokeFamily(ctx, f, models.RevokeLogout, now); err != nil {
			return apperr.Internal(err)
		}
	}
	_ = s.Denylist.DenySessions(ctx, fams...)
	return nil
}

func (s *AuthService) LogoutAll(ctx context.Context, actor *rbac.Actor) error {
	if err := s.RevokeAllSessions(ctx, actor.UserID, nil, models.RevokeLogout); err != nil {
		return err
	}
	return nil
}

// RevokeAllSessions revokes every session of a user (except keep) in DB and Redis.
func (s *AuthService) RevokeAllSessions(ctx context.Context, userID uuid.UUID, keep *uuid.UUID, reason string) error {
	fams, err := s.Repos.Tokens.RevokeUser(ctx, userID, keep, reason, s.now())
	if err != nil {
		return apperr.Internal(err)
	}
	if err := s.Denylist.DenySessions(ctx, fams...); err != nil {
		s.Logger.ErrorContext(ctx, "denylist unavailable", "error", err)
	}
	return nil
}

type SessionInfo struct {
	ID         uuid.UUID  `json:"id"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	UserAgent  string     `json:"user_agent"`
	Current    bool       `json:"current"`
}

func (s *AuthService) Sessions(ctx context.Context, actor *rbac.Actor) ([]SessionInfo, error) {
	rows, err := s.Repos.Tokens.ActiveSessions(ctx, actor.UserID, s.now())
	if err != nil {
		return nil, apperr.Internal(err)
	}
	out := make([]SessionInfo, 0, len(rows))
	for _, t := range rows {
		last := t.LastUsedAt
		if last == nil {
			last = &t.CreatedAt
		}
		out = append(out, SessionInfo{ID: t.FamilyID, CreatedAt: t.CreatedAt, LastUsedAt: last, ExpiresAt: t.ExpiresAt,
			UserAgent: t.UserAgent, Current: t.FamilyID == actor.SessionID})
	}
	return out, nil
}

func (s *AuthService) RevokeSession(ctx context.Context, actor *rbac.Actor, familyID uuid.UUID) error {
	owned, err := s.Repos.Tokens.FamilyOwnedBy(ctx, familyID, actor.UserID)
	if err != nil {
		return apperr.Internal(err)
	}
	if !owned {
		return apperr.NotFound("session")
	}
	if err := s.Repos.Tokens.RevokeFamily(ctx, familyID, models.RevokeLogout, s.now()); err != nil {
		return apperr.Internal(err)
	}
	_ = s.Denylist.DenySessions(ctx, familyID)
	return nil
}

// ChangePassword requires the current password and revokes the other sessions.
func (s *AuthService) ChangePassword(ctx context.Context, actor *rbac.Actor, current, next string) error {
	if res, err := s.Limiter.Allow(ctx, "password_change:"+actor.UserID.String(), LimitPasswordChange...); err == nil && !res.Allowed {
		return apperr.RateLimited(res.RetryAfter)
	}
	u, err := s.Repos.Users.ByID(ctx, actor.UserID)
	if err != nil {
		return apperr.Internal(err)
	}
	if ok, _, _ := s.Hasher.Verify(ctx, current, u.PasswordHash); !ok {
		return apperr.New(apperr.KindForbidden, "invalid_password", "current password is incorrect")
	}
	if code := validators.PasswordLength(next, u.Role == models.RoleAdmin); code != "" {
		return apperr.Invalid("new_password", code)
	}
	if next == current {
		return apperr.Invalid("new_password", "password_unchanged")
	}
	if strings.EqualFold(next, u.Username) {
		return apperr.Invalid("new_password", "password_equals_username")
	}
	if s.Breach.IsBreached(ctx, next) {
		return apperr.Invalid("new_password", "password_breached")
	}
	hash, err := s.Hasher.Hash(ctx, next)
	if err != nil {
		return apperr.Internal(err)
	}
	if err := s.Repos.Users.Update(ctx, u.ID, map[string]any{"password_hash": hash, "must_change_password": false}); err != nil {
		return apperr.Internal(err)
	}
	keep := actor.SessionID
	if err := s.RevokeAllSessions(ctx, u.ID, &keep, models.RevokePasswordChanged); err != nil {
		return err
	}
	return nil
}

// ResetPassword issues a one-time temporary password (RM-07). The caller
// checks that the actor is allowed to reset the target.
func (s *AuthService) ResetPassword(ctx context.Context, target *models.User, reason string) (string, error) {
	temp := crypto.RandomPassword(tempPasswordLength)
	hash, err := s.Hasher.Hash(ctx, temp)
	if err != nil {
		return "", apperr.Internal(err)
	}
	err = s.tx(ctx, func(ctx context.Context) error {
		if err := s.Repos.Users.Update(ctx, target.ID, map[string]any{"password_hash": hash, "must_change_password": true,
			"failed_login_count": 0, "locked_until": nil}); err != nil {
			return apperr.Internal(err)
		}
		if err := s.RevokeAllSessions(ctx, target.ID, nil, models.RevokeAdminPasswordReset); err != nil {
			return err
		}
		s.Alert.Send(ctx, "auth.password.reset", models.SeverityWarn, "user", target.ID.String())
		return nil
	})
	if err != nil {
		return "", err
	}
	return temp, nil
}

// ---------------------------------------------------------------- me

type MeResult struct {
	User              *models.User
	RecoveryRemaining int64
	Establishment     *models.Establishment
	Profile           *models.Profile
}

func (s *AuthService) Me(ctx context.Context, actor *rbac.Actor) (*MeResult, error) {
	u, err := s.Repos.Users.ByID(ctx, actor.UserID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	out := &MeResult{User: u}
	out.RecoveryRemaining, _ = s.Repos.Tokens.RemainingRecoveryCodes(ctx, u.ID)
	switch u.Role {
	case models.RoleManager:
		if e, err := s.Repos.Establishments.ByManager(ctx, u.ID); err == nil {
			out.Establishment = e
		}
	case models.RoleProfessional:
		if p, err := s.Repos.Profiles.ByUser(ctx, u.ID); err == nil {
			out.Profile = p
			if e, err := s.Repos.Establishments.ByID(ctx, p.EstablishmentID); err == nil {
				out.Establishment = e
			}
		}
	}
	return out, nil
}

// ---------------------------------------------------------------- request authentication

// Authenticate verifies an access token (§12.3) and builds the Actor. The
// scope (establishment, profile) is derived from the database, never from
// the request. mutation selects the fail-closed behaviour of the denylist
// when Redis is unavailable (ADR-007).
func (s *AuthService) Authenticate(ctx context.Context, token string, mutation bool) (*rbac.Actor, *models.User, error) {
	claims, err := s.JWT.Verify(token, rbac.AllRoles)
	if err != nil {
		s.Logger.DebugContext(ctx, "token rejected", "reason", err.Error())
		return nil, nil, apperr.Unauthorized("invalid_token")
	}
	userID, _ := uuid.Parse(claims.Subject)
	sid, _ := uuid.Parse(claims.SID)
	denied, err := s.Denylist.IsDenied(ctx, userID, sid)
	if err != nil {
		s.Logger.WarnContext(ctx, "denylist unavailable", "error", err)
		if mutation || claims.Role == models.RoleAdmin {
			return nil, nil, apperr.Unavailable("denylist_unavailable", err)
		}
	} else if denied {
		return nil, nil, apperr.Unauthorized("invalid_token")
	}
	u, err := s.Repos.Users.ByID(ctx, userID)
	if err != nil {
		if database.IsNotFound(err) {
			return nil, nil, apperr.Unauthorized("invalid_token")
		}
		return nil, nil, apperr.Internal(err)
	}
	// The role is never taken from the request; it must match the database.
	if u.Status != models.UserActive || u.Role != claims.Role {
		return nil, nil, apperr.Unauthorized("invalid_token")
	}
	actor := &rbac.Actor{UserID: u.ID, Role: u.Role, SessionID: sid}
	switch u.Role {
	case models.RoleManager:
		e, err := s.Repos.Establishments.ByManager(ctx, u.ID)
		if err != nil {
			if database.IsNotFound(err) {
				return nil, nil, apperr.Unauthorized("invalid_token")
			}
			return nil, nil, apperr.Internal(err)
		}
		actor.EstablishmentID = e.ID
	case models.RoleProfessional:
		p, err := s.Repos.Profiles.ByUser(ctx, u.ID)
		if err != nil {
			if database.IsNotFound(err) {
				return nil, nil, apperr.Unauthorized("invalid_token")
			}
			return nil, nil, apperr.Internal(err)
		}
		active, err := s.employerActive(ctx, u)
		if err != nil {
			return nil, nil, apperr.Internal(err)
		}
		if !active {
			return nil, nil, apperr.Unauthorized("invalid_token")
		}
		actor.ProfileID, actor.EstablishmentID = p.ID, p.EstablishmentID
	}
	return actor, u, nil
}
