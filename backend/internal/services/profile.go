package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/database"
	"opensex/backend/internal/models"
	"opensex/backend/internal/rbac"
	"opensex/backend/internal/repositories"
	"opensex/backend/internal/validators"
	"opensex/backend/pkg/crypto"
	"opensex/backend/pkg/imageproc"
	"opensex/backend/pkg/ratelimit"
)

// ProfileService edits profiles with the ownership rules of RM-20:
// a PROFESSIONAL only reaches their own profile (no identifier accepted), a
// MANAGER reaches every profile of their establishment and nothing else.
type ProfileService struct{ *Services }

const (
	maxPhotos      = 12
	maxRates       = 15
	maxCategories  = 30
	ownerPhotoTTL  = 15 * time.Minute
	publicPhotoTTL = time.Hour
)

var LimitUpload = []ratelimit.Limit{ratelimit.PerHour(60)}

// resolve returns the profile targeted by the actor. For managers the lookup
// is scoped to their establishment; a miss on an existing profile of another
// establishment is recorded as a possible IDOR attempt and answered with 404.
func (s *ProfileService) resolve(ctx context.Context, actor *rbac.Actor, id uuid.UUID, forUpdate bool) (*models.Profile, error) {
	var (
		p   *models.Profile
		err error
	)
	switch actor.Role {
	case models.RoleProfessional:
		if forUpdate {
			p, err = s.Repos.Profiles.ScopedByIDForUpdate(ctx, actor.EstablishmentID, actor.ProfileID)
		} else {
			p, err = s.Repos.Profiles.ScopedByID(ctx, actor.EstablishmentID, actor.ProfileID)
		}
	case models.RoleManager:
		if forUpdate {
			p, err = s.Repos.Profiles.ScopedByIDForUpdate(ctx, actor.EstablishmentID, id)
		} else {
			p, err = s.Repos.Profiles.ScopedByID(ctx, actor.EstablishmentID, id)
		}
	default:
		return nil, apperr.NotFound("profile")
	}
	if err != nil {
		if database.IsNotFound(err) {
			return nil, apperr.NotFound("profile")
		}
		return nil, apperr.Internal(err)
	}
	return p, nil
}

func (s *ProfileService) requireManager(actor *rbac.Actor) error {
	if !actor.Is(models.RoleManager) {
		return apperr.Forbidden("manager_only", "only the establishment manager can do this")
	}
	return nil
}

func (s *ProfileService) uniqueSlug(ctx context.Context, name string) (string, error) {
	base := validators.Slugify(name)
	if base == "" {
		base = "profil"
	}
	for i := 0; i < 5; i++ {
		slug := base + "-" + strings.ToLower(crypto.RandomPassword(6))
		taken, err := s.Repos.Profiles.SlugTaken(ctx, slug)
		if err != nil {
			return "", apperr.Internal(err)
		}
		if !taken {
			return slug, nil
		}
	}
	return "", apperr.Conflict("slug_unavailable", "could not allocate a slug")
}

// ---------------------------------------------------------------- read

type PhotoView struct {
	models.ProfilePhoto
	URL string
}

type ProfileDetail struct {
	Profile       *models.Profile
	Photos        []PhotoView
	Contacts      []models.ProfileContact
	Rates         []models.ProfileRate
	Categories    []models.Category
	Establishment *models.Establishment
	LinkedUser    *models.User
	LastEditor    *models.User
	Reasons       []string
}

func (s *ProfileService) photoURL(ctx context.Context, key string, public bool) string {
	if public {
		if u := s.Storage.PublicURL(key); u != "" {
			return u
		}
		u, _ := s.Storage.SignedURL(ctx, key, publicPhotoTTL)
		return u
	}
	u, _ := s.Storage.SignedURL(ctx, key, ownerPhotoTTL)
	return u
}

func (s *ProfileService) detail(ctx context.Context, actor *rbac.Actor, p *models.Profile) (*ProfileDetail, error) {
	d := &ProfileDetail{Profile: p}
	var err error
	photos, err := s.Repos.Profiles.Photos(ctx, p.ID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	for _, ph := range photos {
		d.Photos = append(d.Photos, PhotoView{ProfilePhoto: ph, URL: s.photoURL(ctx, ph.StorageKey, false)})
	}
	if d.Contacts, err = s.Repos.Profiles.Contacts(ctx, p.ID); err != nil {
		return nil, apperr.Internal(err)
	}
	if d.Rates, err = s.Repos.Profiles.Rates(ctx, p.ID); err != nil {
		return nil, apperr.Internal(err)
	}
	if d.Categories, err = s.Repos.Profiles.Categories(ctx, p.ID); err != nil {
		return nil, apperr.Internal(err)
	}
	if d.Establishment, err = s.Repos.Establishments.ByID(ctx, p.EstablishmentID); err != nil {
		return nil, apperr.Internal(err)
	}
	if p.UserID != nil {
		if u, err := s.Repos.Users.ByID(ctx, *p.UserID); err == nil {
			d.LinkedUser = u
		}
	}
	if p.LastEditedBy != nil {
		if u, err := s.Repos.Users.ByID(ctx, *p.LastEditedBy); err == nil {
			d.LastEditor = u
		}
	}
	st, err := s.Publication.EstablishmentState(ctx, d.Establishment)
	if err != nil {
		return nil, err
	}
	if d.Reasons, err = s.Publication.ProfileReasons(ctx, st, p); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *ProfileService) Get(ctx context.Context, actor *rbac.Actor, id uuid.UUID) (*ProfileDetail, error) {
	p, err := s.resolve(ctx, actor, id, false)
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, actor, p)
}

type ProfileListItem struct {
	Profile    models.Profile
	LinkedUser *models.User
	LastEditor *models.User
	Reasons    []string
	PrimaryURL string
}

// List returns the profiles of the manager's establishment.
func (s *ProfileService) List(ctx context.Context, actor *rbac.Actor, f repositories.ProfileFilter, page repositories.Page) ([]ProfileListItem, int64, error) {
	f.EstablishmentID = &actor.EstablishmentID
	rows, total, err := s.Repos.Profiles.List(ctx, f, page)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	e, err := s.Repos.Establishments.ByID(ctx, actor.EstablishmentID)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	st, err := s.Publication.EstablishmentState(ctx, e)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]uuid.UUID, len(rows))
	for i, p := range rows {
		ids[i] = p.ID
	}
	primaries, err := s.Repos.Profiles.PrimaryPhotos(ctx, ids)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	users := map[uuid.UUID]*models.User{}
	loadUser := func(id *uuid.UUID) *models.User {
		if id == nil {
			return nil
		}
		if u, ok := users[*id]; ok {
			return u
		}
		u, err := s.Repos.Users.ByID(ctx, *id)
		if err != nil {
			u = nil
		}
		users[*id] = u
		return u
	}
	out := make([]ProfileListItem, len(rows))
	for i := range rows {
		p := rows[i]
		reasons, err := s.Publication.ProfileReasons(ctx, st, &p)
		if err != nil {
			return nil, 0, err
		}
		item := ProfileListItem{Profile: p, LinkedUser: loadUser(p.UserID), LastEditor: loadUser(p.LastEditedBy), Reasons: reasons}
		if ph, ok := primaries[p.ID]; ok {
			item.PrimaryURL = s.photoURL(ctx, ph.StorageKey, false)
		}
		out[i] = item
	}
	return out, total, nil
}

// ---------------------------------------------------------------- content

type ProfileInput struct {
	DisplayName     string
	Headline        string
	Description     string
	Age             *int
	HeightCm        *int
	WeightKg        *int
	HairColor       *string
	EyeColor        *string
	BodyType        *string
	Languages       []string
	ExtraAttributes map[string]string
}

func validateProfileInput(in ProfileInput) (map[string]any, error) {
	var c apperr.Collector
	cols := map[string]any{}
	name, code := validators.PlainText(in.DisplayName, 60)
	if code != "" || name == "" {
		c.Add("display_name", firstNonEmpty(code, "required"))
	}
	cols["display_name"] = name
	headline, code := validators.PlainText(in.Headline, 120)
	if code != "" {
		c.Add("headline", code)
	}
	cols["headline"] = headline
	desc, code := validators.PlainText(in.Description, 5000)
	if code != "" {
		c.Add("description", code)
	}
	cols["description"] = desc
	if in.Age != nil && (*in.Age < 18 || *in.Age > 99) {
		c.Add("age", "out_of_range")
	}
	cols["age"] = in.Age
	if in.HeightCm != nil && (*in.HeightCm < 120 || *in.HeightCm > 230) {
		c.Add("height_cm", "out_of_range")
	}
	cols["height_cm"] = in.HeightCm
	if in.WeightKg != nil && (*in.WeightKg < 35 || *in.WeightKg > 250) {
		c.Add("weight_kg", "out_of_range")
	}
	cols["weight_kg"] = in.WeightKg
	for field, pair := range map[string]struct {
		v       *string
		allowed []string
	}{"hair_color": {in.HairColor, validators.HairColors}, "eye_color": {in.EyeColor, validators.EyeColors}, "body_type": {in.BodyType, validators.BodyTypes}} {
		if pair.v != nil && !validators.OneOf(*pair.v, pair.allowed) {
			c.Add(field, "invalid_value")
		}
		cols[field] = pair.v
	}
	langs := models.StringArray{}
	seen := map[string]bool{}
	if len(in.Languages) > 15 {
		c.Add("languages", "too_many")
	}
	for _, l := range in.Languages {
		l = strings.ToLower(strings.TrimSpace(l))
		if !validators.Language(l) {
			c.Add("languages", "invalid_language")
			break
		}
		if !seen[l] {
			langs = append(langs, l)
			seen[l] = true
		}
	}
	cols["languages"] = langs
	extra, code := validators.ValidateExtraAttributes(in.ExtraAttributes)
	if code != "" {
		c.Add("extra_attributes", code)
	}
	cols["extra_attributes"] = models.JSON(extra)
	return cols, c.Err()
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// AccountInput holds the credentials a manager gives to a professional.
type AccountInput struct {
	Username string
	Password string
}

// newProfessionalAccount validates and hashes the credentials chosen by the
// manager. The account is created inside the caller's transaction.
func (s *ProfileService) newProfessionalAccount(ctx context.Context, acc AccountInput) (*models.User, error) {
	in := RegisterInput{Username: strings.TrimSpace(acc.Username), Password: acc.Password}
	if err := s.Auth.checkCredentials(ctx, in, false); err != nil {
		return nil, err
	}
	u, err := s.Auth.newUser(ctx, in)
	if err != nil {
		return nil, err
	}
	now := s.now()
	u.RecoveryCodesConfirmedAt = &now // no recovery codes: the manager resets the password
	return u, nil
}

// CreateByManager creates a profile of the house, with an optional login for
// the professional (ADR-022). Professionals never register by themselves.
// With a paid subscription the profile is in service at once (ADR-021).
func (s *ProfileService) CreateByManager(ctx context.Context, actor *rbac.Actor, in ProfileInput, account *AccountInput) (*ProfileDetail, error) {
	if err := s.requireManager(actor); err != nil {
		return nil, err
	}
	cols, err := validateProfileInput(in)
	if err != nil {
		return nil, err
	}
	var user *models.User
	if account != nil {
		if user, err = s.newProfessionalAccount(ctx, *account); err != nil {
			return nil, err
		}
	}
	var out *ProfileDetail
	err = s.tx(ctx, func(ctx context.Context) error {
		slug, err := s.uniqueSlug(ctx, cols["display_name"].(string))
		if err != nil {
			return err
		}
		now := s.now()
		entitled, _, err := s.Repos.Profiles.HasEntitlement(ctx, actor.EstablishmentID, now)
		if err != nil {
			return apperr.Internal(err)
		}
		status := models.ProfileDraft
		if entitled {
			status = models.ProfileActive
		}
		p := &models.Profile{ID: models.NewID(), EstablishmentID: actor.EstablishmentID, Slug: slug, Status: status,
			Languages: models.StringArray{}, ExtraAttributes: models.JSON("{}"), LastEditedBy: &actor.UserID, CreatedAt: now, UpdatedAt: now,
			DisplayName: cols["display_name"].(string)}
		if user != nil {
			if err := s.Auth.createUser(ctx, user, models.RoleProfessional); err != nil {
				return err
			}
			p.UserID = &user.ID
		}
		if err := s.Repos.Profiles.Create(ctx, p); err != nil {
			return apperr.Internal(err)
		}
		cols["last_edited_by"] = actor.UserID
		if err := s.Repos.Profiles.Update(ctx, p.ID, cols); err != nil {
			return apperr.Internal(err)
		}
		// A profile created ACTIVE by a subscribed house is published at once.
		if err := s.Publication.Recompute(ctx, p.EstablishmentID); err != nil {
			return err
		}
		p, err = s.Repos.Profiles.ByID(ctx, p.ID)
		if err != nil {
			return apperr.Internal(err)
		}
		out, err = s.detail(ctx, actor, p)
		return err
	})
	return out, err
}

// CreateAccount gives a login to a profile of the house that has none yet.
func (s *ProfileService) CreateAccount(ctx context.Context, actor *rbac.Actor, id uuid.UUID, account AccountInput) (*ProfileDetail, error) {
	if err := s.requireManager(actor); err != nil {
		return nil, err
	}
	user, err := s.newProfessionalAccount(ctx, account)
	if err != nil {
		return nil, err
	}
	var out *ProfileDetail
	err = s.tx(ctx, func(ctx context.Context) error {
		p, err := s.resolve(ctx, actor, id, true)
		if err != nil {
			return err
		}
		if p.UserID != nil {
			return apperr.Conflict("account_exists", "this profile already has a login")
		}
		if err := s.Auth.createUser(ctx, user, models.RoleProfessional); err != nil {
			return err
		}
		if err := s.Repos.Profiles.Update(ctx, p.ID, map[string]any{"user_id": user.ID, "last_edited_by": actor.UserID}); err != nil {
			return apperr.Internal(err)
		}
		if p, err = s.Repos.Profiles.ByID(ctx, p.ID); err != nil {
			return apperr.Internal(err)
		}
		out, err = s.detail(ctx, actor, p)
		return err
	})
	return out, err
}

// Update replaces the editable content of a profile (owner or manager, RM-24).
func (s *ProfileService) Update(ctx context.Context, actor *rbac.Actor, id uuid.UUID, in ProfileInput) (*ProfileDetail, error) {
	cols, err := validateProfileInput(in)
	if err != nil {
		return nil, err
	}
	var out *ProfileDetail
	err = s.tx(ctx, func(ctx context.Context) error {
		p, err := s.resolve(ctx, actor, id, true)
		if err != nil {
			return err
		}
		if p.DisplayName != cols["display_name"] {
			if cols["slug"], err = s.uniqueSlug(ctx, cols["display_name"].(string)); err != nil {
				return err
			}
		}
		cols["last_edited_by"] = actor.UserID
		if err := s.Repos.Profiles.Update(ctx, p.ID, cols); err != nil {
			return apperr.Internal(err)
		}
		if p, err = s.Repos.Profiles.ByID(ctx, p.ID); err != nil {
			return apperr.Internal(err)
		}
		out, err = s.detail(ctx, actor, p)
		return err
	})
	return out, err
}

// touch marks the profile as edited by the actor.
func (s *ProfileService) touch(ctx context.Context, actor *rbac.Actor, p *models.Profile, extra map[string]any) error {
	cols := map[string]any{"last_edited_by": actor.UserID}
	for k, v := range extra {
		cols[k] = v
	}
	if err := s.Repos.Profiles.Update(ctx, p.ID, cols); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (s *ProfileService) ReplaceCategories(ctx context.Context, actor *rbac.Actor, id uuid.UUID, ids []uuid.UUID) ([]models.Category, error) {
	uniq := uniqueIDs(ids)
	if len(uniq) > maxCategories {
		return nil, apperr.Invalid("category_ids", "too_many")
	}
	var out []models.Category
	err := s.tx(ctx, func(ctx context.Context) error {
		p, err := s.resolve(ctx, actor, id, true)
		if err != nil {
			return err
		}
		cats, err := s.Repos.Taxonomy.SelectableCategories(ctx, uniq)
		if err != nil {
			return apperr.Internal(err)
		}
		if len(cats) != len(uniq) {
			return apperr.Invalid("category_ids", "unknown_category")
		}
		if err := s.Repos.Profiles.ReplaceCategories(ctx, p.ID, uniq); err != nil {
			return apperr.Internal(err)
		}
		if err := s.touch(ctx, actor, p, nil); err != nil {
			return err
		}
		out, err = s.Repos.Profiles.Categories(ctx, p.ID)
		return err
	})
	return out, err
}

func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	out := []uuid.UUID{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

type RateInput struct {
	Label           string
	DurationMinutes *int
	AmountCents     int64
}

// ReplaceRates stores 1 to 15 rates and refreshes price_from_cents (RM-25).
func (s *ProfileService) ReplaceRates(ctx context.Context, actor *rbac.Actor, id uuid.UUID, in []RateInput) ([]models.ProfileRate, error) {
	var c apperr.Collector
	if len(in) < 1 || len(in) > maxRates {
		c.Add("rates", "must_contain_1_to_15_items")
	}
	var min int64
	for i, r := range in {
		f := "rates[" + itoa(i) + "]"
		label, code := validators.PlainText(r.Label, 60)
		if code != "" || label == "" {
			c.Add(f+".label", firstNonEmpty(code, "required"))
		}
		in[i].Label = label
		if r.DurationMinutes != nil && (*r.DurationMinutes < 1 || *r.DurationMinutes > 1440) {
			c.Add(f+".duration_minutes", "out_of_range")
		}
		if r.AmountCents <= 0 || r.AmountCents > 10_000_000 {
			c.Add(f+".amount_cents", "out_of_range")
		}
		if min == 0 || r.AmountCents < min {
			min = r.AmountCents
		}
	}
	if err := c.Err(); err != nil {
		return nil, err
	}
	var out []models.ProfileRate
	err := s.tx(ctx, func(ctx context.Context) error {
		p, err := s.resolve(ctx, actor, id, true)
		if err != nil {
			return err
		}
		rows := make([]models.ProfileRate, len(in))
		for i, r := range in {
			rows[i] = models.ProfileRate{ID: models.NewID(), ProfileID: p.ID, Label: r.Label, DurationMinutes: r.DurationMinutes,
				AmountCents: r.AmountCents, Currency: "EUR", Position: i}
		}
		if err := s.Repos.Profiles.ReplaceRates(ctx, p.ID, rows); err != nil {
			return apperr.Internal(err)
		}
		if err := s.touch(ctx, actor, p, map[string]any{"price_from_cents": min}); err != nil {
			return err
		}
		out = rows
		return nil
	})
	return out, err
}

func (s *ProfileService) ReplaceContacts(ctx context.Context, actor *rbac.Actor, id uuid.UUID, in []ContactInput) ([]models.ProfileContact, error) {
	norm, err := normalizeContacts(in, false)
	if err != nil {
		return nil, err
	}
	var out []models.ProfileContact
	err = s.tx(ctx, func(ctx context.Context) error {
		p, err := s.resolve(ctx, actor, id, true)
		if err != nil {
			return err
		}
		rows := make([]models.ProfileContact, len(norm))
		for i, ct := range norm {
			rows[i] = models.ProfileContact{ID: models.NewID(), ProfileID: p.ID, Type: ct.Type, Value: ct.Value, Visibility: ct.Visibility, Position: i}
		}
		if err := s.Repos.Profiles.ReplaceContacts(ctx, p.ID, rows); err != nil {
			return apperr.Internal(err)
		}
		if err := s.touch(ctx, actor, p, nil); err != nil {
			return err
		}
		out = rows
		return nil
	})
	return out, err
}

// ---------------------------------------------------------------- photos

func (s *ProfileService) UploadPhoto(ctx context.Context, actor *rbac.Actor, id uuid.UUID, data []byte) (*PhotoView, error) {
	p, err := s.resolve(ctx, actor, id, false)
	if err != nil {
		return nil, err
	}
	if res, err := s.Limiter.Allow(ctx, "upload:"+p.EstablishmentID.String(), LimitUpload...); err == nil && !res.Allowed {
		return nil, apperr.RateLimited(res.RetryAfter)
	}
	if n, err := s.Repos.Profiles.CountPhotos(ctx, p.ID); err != nil {
		return nil, apperr.Internal(err)
	} else if n >= maxPhotos {
		return nil, apperr.Unprocessable("too_many_photos", "a profile has at most 12 photos")
	}
	img, err := imageproc.Process(data)
	if err != nil {
		code := "invalid_image"
		switch {
		case errors.Is(err, imageproc.ErrUnsupportedFormat):
			code = "unsupported_image_format"
		case errors.Is(err, imageproc.ErrTooLarge):
			code = "image_too_large"
		}
		return nil, apperr.Invalid("file", code)
	}
	photo := &models.ProfilePhoto{ID: models.NewID(), ProfileID: p.ID, Width: img.Width, Height: img.Height, CreatedAt: s.now()}
	photo.StorageKey = "profiles/" + p.ID.String() + "/" + photo.ID.String() + ".jpg"
	if err := s.Storage.Put(ctx, photo.StorageKey, img.Data, img.ContentType); err != nil {
		return nil, apperr.Unavailable("storage_unavailable", err)
	}
	err = s.tx(ctx, func(ctx context.Context) error {
		p, err := s.resolve(ctx, actor, id, true)
		if err != nil {
			return err
		}
		n, err := s.Repos.Profiles.CountPhotos(ctx, p.ID)
		if err != nil {
			return apperr.Internal(err)
		}
		if n >= maxPhotos {
			return apperr.Unprocessable("too_many_photos", "a profile has at most 12 photos")
		}
		photo.Position = int(n)
		photo.IsPrimary = n == 0
		if err := s.Repos.Profiles.CreatePhoto(ctx, photo); err != nil {
			return apperr.Internal(err)
		}
		if err := s.touch(ctx, actor, p, nil); err != nil {
			return err
		}
		if photo.IsPrimary {
			return s.Publication.Recompute(ctx, p.EstablishmentID)
		}
		return nil
	})
	if err != nil {
		_ = s.Storage.Delete(context.WithoutCancel(ctx), photo.StorageKey)
		return nil, err
	}
	return &PhotoView{ProfilePhoto: *photo, URL: s.photoURL(ctx, photo.StorageKey, false)}, nil
}

type PhotoPatch struct {
	IsPrimary *bool
	Position  *int
}

func (s *ProfileService) UpdatePhoto(ctx context.Context, actor *rbac.Actor, id, photoID uuid.UUID, in PhotoPatch) ([]PhotoView, error) {
	var out []PhotoView
	err := s.tx(ctx, func(ctx context.Context) error {
		p, err := s.resolve(ctx, actor, id, true)
		if err != nil {
			return err
		}
		if _, err := s.Repos.Profiles.PhotoScoped(ctx, p.ID, photoID); err != nil {
			if database.IsNotFound(err) {
				return apperr.NotFound("photo")
			}
			return apperr.Internal(err)
		}
		if in.Position != nil {
			if *in.Position < 0 || *in.Position >= maxPhotos {
				return apperr.Invalid("position", "out_of_range")
			}
			if err := s.Repos.Profiles.SetPhotoPosition(ctx, p.ID, photoID, *in.Position); err != nil {
				return apperr.Internal(err)
			}
		}
		if in.IsPrimary != nil && *in.IsPrimary {
			if err := s.Repos.Profiles.SetPrimaryPhoto(ctx, p.ID, photoID); err != nil {
				return apperr.Internal(err)
			}
		}
		if err := s.touch(ctx, actor, p, nil); err != nil {
			return err
		}
		out, err = s.photoViews(ctx, p.ID)
		return err
	})
	return out, err
}

func (s *ProfileService) ReorderPhotos(ctx context.Context, actor *rbac.Actor, id uuid.UUID, order []uuid.UUID) ([]PhotoView, error) {
	var out []PhotoView
	err := s.tx(ctx, func(ctx context.Context) error {
		p, err := s.resolve(ctx, actor, id, true)
		if err != nil {
			return err
		}
		photos, err := s.Repos.Profiles.Photos(ctx, p.ID)
		if err != nil {
			return apperr.Internal(err)
		}
		uniq := uniqueIDs(order)
		if len(uniq) != len(photos) || len(order) != len(photos) {
			return apperr.Invalid("photo_ids", "must_list_every_photo_once")
		}
		owned := map[uuid.UUID]bool{}
		for _, ph := range photos {
			owned[ph.ID] = true
		}
		for i, pid := range order {
			if !owned[pid] {
				return apperr.NotFound("photo")
			}
			if err := s.Repos.Profiles.SetPhotoPosition(ctx, p.ID, pid, i); err != nil {
				return apperr.Internal(err)
			}
		}
		if err := s.touch(ctx, actor, p, nil); err != nil {
			return err
		}
		out, err = s.photoViews(ctx, p.ID)
		return err
	})
	return out, err
}

func (s *ProfileService) DeletePhoto(ctx context.Context, actor *rbac.Actor, id, photoID uuid.UUID) error {
	p, err := s.resolve(ctx, actor, id, false)
	if err != nil {
		return err
	}
	return s.removePhoto(ctx, p, photoID, func(ctx context.Context) error { return s.touch(ctx, actor, p, nil) })
}

// removePhoto deletes a photo, promotes the next one as primary if needed,
// recomputes the publication and removes the object from storage.
func (s *ProfileService) removePhoto(ctx context.Context, p *models.Profile, photoID uuid.UUID, extra func(ctx context.Context) error) error {
	var key string
	err := s.tx(ctx, func(ctx context.Context) error {
		if _, err := s.Repos.Profiles.ScopedByIDForUpdate(ctx, p.EstablishmentID, p.ID); err != nil {
			return apperr.NotFound("profile")
		}
		ph, err := s.Repos.Profiles.PhotoScoped(ctx, p.ID, photoID)
		if err != nil {
			if database.IsNotFound(err) {
				return apperr.NotFound("photo")
			}
			return apperr.Internal(err)
		}
		key = ph.StorageKey
		if err := s.Repos.Profiles.DeletePhoto(ctx, ph.ID); err != nil {
			return apperr.Internal(err)
		}
		if ph.IsPrimary {
			rest, err := s.Repos.Profiles.Photos(ctx, p.ID)
			if err != nil {
				return apperr.Internal(err)
			}
			if len(rest) > 0 {
				if err := s.Repos.Profiles.SetPrimaryPhoto(ctx, p.ID, rest[0].ID); err != nil {
					return apperr.Internal(err)
				}
			}
		}
		if extra != nil {
			if err := extra(ctx); err != nil {
				return err
			}
		}
		return s.Publication.Recompute(ctx, p.EstablishmentID)
	})
	if err != nil {
		return err
	}
	if err := s.Storage.Delete(context.WithoutCancel(ctx), key); err != nil {
		s.Logger.WarnContext(ctx, "photo object deletion failed", "key", key, "error", err)
	}
	return nil
}

func (s *ProfileService) photoViews(ctx context.Context, profileID uuid.UUID) ([]PhotoView, error) {
	photos, err := s.Repos.Profiles.Photos(ctx, profileID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	out := make([]PhotoView, len(photos))
	for i, ph := range photos {
		out[i] = PhotoView{ProfilePhoto: ph, URL: s.photoURL(ctx, ph.StorageKey, false)}
	}
	return out, nil
}

// ---------------------------------------------------------------- manager-only lifecycle

// Activate moves a DRAFT or ARCHIVED profile to ACTIVE.
func (s *ProfileService) Activate(ctx context.Context, actor *rbac.Actor, id uuid.UUID) (*ProfileDetail, error) {
	if err := s.requireManager(actor); err != nil {
		return nil, err
	}
	var out *ProfileDetail
	err := s.tx(ctx, func(ctx context.Context) error {
		p, err := s.resolve(ctx, actor, id, true)
		if err != nil {
			return err
		}
		if p.Status != models.ProfileDraft && p.Status != models.ProfileArchived {
			return apperr.Conflict("invalid_state", "profile cannot be activated from status "+p.Status)
		}
		if err := s.Repos.Profiles.Update(ctx, p.ID, map[string]any{"status": models.ProfileActive, "last_edited_by": actor.UserID}); err != nil {
			return apperr.Internal(err)
		}
		if err := s.Publication.Recompute(ctx, p.EstablishmentID); err != nil {
			return err
		}
		if p, err = s.Repos.Profiles.ByID(ctx, p.ID); err != nil {
			return apperr.Internal(err)
		}
		out, err = s.detail(ctx, actor, p)
		return err
	})
	return out, err
}

// Archive unpublishes the profile and suspends
// the linked professional account (RM-24).
func (s *ProfileService) Archive(ctx context.Context, actor *rbac.Actor, id uuid.UUID) error {
	if err := s.requireManager(actor); err != nil {
		return err
	}
	var linked *uuid.UUID
	err := s.tx(ctx, func(ctx context.Context) error {
		p, err := s.resolve(ctx, actor, id, true)
		if err != nil {
			return err
		}
		if p.Status == models.ProfileArchived {
			return apperr.Conflict("invalid_state", "profile already archived")
		}
		cols := map[string]any{"status": models.ProfileArchived, "last_edited_by": actor.UserID}
		if err := s.Repos.Profiles.Update(ctx, p.ID, cols); err != nil {
			return apperr.Internal(err)
		}
		if p.UserID != nil {
			linked = p.UserID
			if err := s.Repos.Users.Update(ctx, *p.UserID, map[string]any{"status": models.UserSuspended, "status_reason": "profile archived"}); err != nil {
				return apperr.Internal(err)
			}
			if err := s.Auth.RevokeAllSessions(ctx, *p.UserID, nil, models.RevokeSuspended); err != nil {
				return err
			}
		}
		return s.Publication.Recompute(ctx, p.EstablishmentID)
	})
	if err == nil && linked != nil {
		_ = s.Denylist.DenyUser(ctx, *linked)
	}
	return err
}

// Delete removes a profile (soft delete, purged after 30 days) and deletes
// the linked professional account.
func (s *ProfileService) Delete(ctx context.Context, actor *rbac.Actor, id uuid.UUID) error {
	if err := s.requireManager(actor); err != nil {
		return err
	}
	var linked *uuid.UUID
	err := s.tx(ctx, func(ctx context.Context) error {
		p, err := s.resolve(ctx, actor, id, true)
		if err != nil {
			return err
		}
		if err := s.Repos.Profiles.SoftDelete(ctx, p.ID, s.now()); err != nil {
			return apperr.Internal(err)
		}
		if p.UserID != nil {
			linked = p.UserID
			if err := s.Clients.deleteAccount(ctx, *p.UserID); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil && linked != nil {
		_ = s.Denylist.DenyUser(ctx, *linked)
	}
	return err
}

// ---------------------------------------------------------------- status & preview

type PublicationStatus struct {
	Published   bool       `json:"published"`
	PublishedAt *time.Time `json:"published_at"`
	Status      string     `json:"status"`
	Reasons     []string   `json:"reasons"`
}

func (s *ProfileService) PublicationStatus(ctx context.Context, actor *rbac.Actor) (*PublicationStatus, error) {
	d, err := s.Get(ctx, actor, actor.ProfileID)
	if err != nil {
		return nil, err
	}
	return &PublicationStatus{Published: d.Profile.IsPublished, PublishedAt: d.Profile.PublishedAt, Status: d.Profile.Status,
		Reasons: d.Reasons}, nil
}

// Preview renders the profile as the public would see it (contacts except HIDDEN).
func (s *ProfileService) Preview(ctx context.Context, actor *rbac.Actor, id uuid.UUID) (*PublicProfile, error) {
	p, err := s.resolve(ctx, actor, id, false)
	if err != nil {
		return nil, err
	}
	return s.Catalog.build(ctx, p, viewPreview)
}
