package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"opensex/backend/internal/database"
	"opensex/backend/internal/models"
	"opensex/backend/internal/validators"
)

// BootstrapService creates the platform administrators at API startup
// (RM-02, RM-02b). It never promotes or modifies an existing account.
type BootstrapService struct{ *Services }

type adminSpec struct {
	Username     string `json:"username"`
	PasswordFile string `json:"password_file"`
	password     string
}

// EnsureBootstrapAdmins runs before the HTTP port is opened; any error must
// abort the startup.
func (s *BootstrapService) EnsureBootstrapAdmins(ctx context.Context) error {
	var specs []adminSpec
	if s.Cfg.AdminBootstrapUsername != "" {
		if s.Cfg.AdminBootstrapPassword == "" {
			return fmt.Errorf("ADMIN_BOOTSTRAP_USERNAME is set but no password was provided (ADMIN_BOOTSTRAP_PASSWORD_FILE)")
		}
		specs = append(specs, adminSpec{Username: s.Cfg.AdminBootstrapUsername, password: s.Cfg.AdminBootstrapPassword})
	}
	if f := s.Cfg.AdminBootstrapUsersFile; f != "" {
		more, err := readAdminSpecs(f)
		if err != nil {
			return err
		}
		specs = append(specs, more...)
	}
	for _, spec := range specs {
		if err := s.ensureOne(ctx, spec); err != nil {
			return fmt.Errorf("bootstrap admin %q: %w", spec.Username, err)
		}
	}
	return nil
}

// readAdminSpecs reads a JSON array [{"username": "...", "password_file": "..."}];
// relative password files are resolved against the users file directory.
func readAdminSpecs(path string) ([]adminSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ADMIN_BOOTSTRAP_USERS_FILE: %w", err)
	}
	var specs []adminSpec
	if err := json.Unmarshal(data, &specs); err != nil {
		return nil, fmt.Errorf("ADMIN_BOOTSTRAP_USERS_FILE: %w", err)
	}
	for i := range specs {
		pf := specs[i].PasswordFile
		if pf == "" {
			return nil, fmt.Errorf("ADMIN_BOOTSTRAP_USERS_FILE: entry %d has no password_file", i)
		}
		if !filepath.IsAbs(pf) {
			pf = filepath.Join(filepath.Dir(path), pf)
		}
		b, err := os.ReadFile(pf)
		if err != nil {
			return nil, fmt.Errorf("ADMIN_BOOTSTRAP_USERS_FILE: %w", err)
		}
		specs[i].password = strings.TrimSpace(string(b))
	}
	return specs, nil
}

func (s *BootstrapService) ensureOne(ctx context.Context, spec adminSpec) error {
	if code := validators.Username(spec.Username); code != "" && code != "reserved_username" {
		return fmt.Errorf("invalid username (%s)", code)
	}
	if code := validators.PasswordLength(spec.password, true); code != "" {
		return fmt.Errorf("password rejected (%s): at least 16 characters are required", code)
	}
	if s.Breach.IsBreached(ctx, spec.password) {
		return fmt.Errorf("password rejected: known compromised password")
	}
	release, err := s.DB.Lock(ctx, "bootstrap-admin")
	if err != nil {
		return err
	}
	defer release()
	existing, err := s.Repos.Users.ByUsername(ctx, spec.Username)
	if err == nil {
		if existing.Role != models.RoleAdmin {
			return fmt.Errorf("username already used by a non-admin account; refusing to promote it")
		}
		return nil // idempotent: never modify an existing admin
	}
	if !database.IsNotFound(err) {
		return err
	}
	if taken, err := s.Repos.Users.Exists(ctx, spec.Username); err != nil {
		return err
	} else if taken {
		return fmt.Errorf("username already used by a deleted account; refusing to reuse it")
	}
	hash, err := s.Hasher.Hash(ctx, spec.password)
	if err != nil {
		return err
	}
	now := s.now()
	u := &models.User{ID: models.NewID(), Username: spec.Username, PasswordHash: hash, Status: models.UserActive,
		MustChangePassword: true, TermsVersionAccepted: "bootstrap", TermsAcceptedAt: now, RecoveryCodesConfirmedAt: &now,
		CreatedAt: now, UpdatedAt: now}
	return s.tx(ctx, func(ctx context.Context) error {
		if err := s.Repos.Users.Create(ctx, u, models.RoleAdmin); err != nil {
			return err
		}
		s.Alert.Send(ctx, "admin.bootstrap.created", models.SeverityWarn, "user", u.ID.String())
		s.Logger.Warn("bootstrap admin created; the password must be changed at first login", "username", spec.Username)
		return nil
	})
}
