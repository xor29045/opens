// Package devseed fills a development database with realistic demo data so
// that every dashboard (public catalogue, manager, professional,
// admin) has something to show. Everything goes through the business
// services: passwords are hashed, photos re-encoded and stored, payments are
// confirmed by signed IPNs of the fake provider.
//
// It refuses to run outside development and without the fake payment provider.
package devseed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math/rand"
	"time"

	"github.com/google/uuid"

	"opensex/backend/internal/config"
	"opensex/backend/internal/models"
	"opensex/backend/internal/rbac"
	"opensex/backend/internal/services"
	"opensex/backend/pkg/payment"
)

// Password shared by every demo account.
const Password = "demo opensex passphrase"

// Marker username: when it exists the database is considered seeded.
const marker = "rose_gerant"

var ErrAlreadySeeded = errors.New("demo data already present (user " + marker + " exists)")

type Report struct {
	Establishments int
	Profiles       int
	Drafts         int
}

type house struct {
	username, name, legal, city, region, address, description string
	subscription                                              string // paid | past_due | renewal_due | none
	profiles, pending                                         int
}

var houses = []house{
	{"rose_gerant", "Maison Rose", "Maison Rose SRL", "ixelles", "bruxelles-capitale", "Rue du Bailli 12, 1050 Ixelles",
		"Une maison chaleureuse au cœur d’Ixelles. Accueil discret, salons feutrés et une équipe attentive.", "paid", 6, 2},
	{"velours_gerant", "Le Velours", "Velours & Co SRL", "bruxelles", "bruxelles-capitale", "Rue Antoine Dansaert 40, 1000 Bruxelles",
		"Ambiance tamisée, musique douce et un service à l’écoute, à deux pas de la Bourse.", "renewal_due", 5, 1},
	{"lune_gerant", "Villa Lune", "Villa Lune SRL", "anvers", "flandre", "Lange Nieuwstraat 8, 2000 Antwerpen",
		"Une villa élégante près de la cathédrale. Jardin intérieur et suites privées.", "paid", 5, 0},
	{"jade_gerant", "Le Jardin de Jade", "Jardin de Jade SRL", "gand", "flandre", "Veldstraat 21, 9000 Gent",
		"Inspiration zen et massages. Une parenthèse calme au centre de Gand.", "paid", 4, 1},
	{"opale_gerant", "L’Opale", "Opale Liège SRL", "liege", "wallonie", "Rue Pont d’Avroy 5, 4000 Liège",
		"La maison liégeoise des soirées raffinées.", "past_due", 4, 0},
	{"nacre_gerant", "Maison Nacre", "Nacre SRL", "namur", "wallonie", "",
		"Nouvelle maison namuroise, ouverture prochaine.", "none", 3, 2},
}

var names = []string{"Lola", "Mia", "Nina", "Jade", "Inès", "Clara", "Zoé", "Léa", "Sofia", "Eva", "Maya", "Alba", "Lina", "Romy",
	"Chloé", "Elena", "Victoria", "Alice", "Emma", "Julia", "Luna", "Noor", "Sasha", "Kim", "Andrea", "Leo", "Camille", "Iris",
	"Margaux", "Paloma", "Vera", "Anouk", "Selena", "Ambre"}

var headlines = []string{"Douce, souriante et attentionnée", "Pétillante et pleine d’humour", "Élégante et raffinée",
	"Zen, à l’écoute, adepte des massages", "Spontanée et chaleureuse", "Mystérieuse et envoûtante", "Complice et naturelle",
	"Raffinement et discrétion", "Énergie solaire", "Toute en délicatesse"}

var descriptions = []string{
	"J’aime prendre le temps de faire connaissance. Un moment de détente, sans précipitation, dans le respect de chacun·e.",
	"Bavarde, curieuse et toujours de bonne humeur. Je parle plusieurs langues et j’adore les rencontres sincères.",
	"Je privilégie l’écoute et la douceur. Massages relaxants et moments complices dans une ambiance apaisante.",
	"Passionnée de danse et de voyages, j’aime les échanges élégants et les soirées qui se prolongent.",
	"Naturelle et attentive, je m’adapte à vos envies dans le respect de mes limites. Rendez-vous à la maison.",
}

var palette = []color.RGBA{{0xb5, 0x2f, 0x58, 255}, {0x2e, 0x6b, 0x8a, 255}, {0x6b, 0x5b, 0x2e, 255}, {0x4a, 0x3b, 0x6e, 255},
	{0x2f, 0x6e, 0x5a, 255}, {0x8a, 0x4b, 0x2e, 255}, {0x3d, 0x4f, 0x7a, 255}, {0x7a, 0x3d, 0x5c, 255}}

// Run seeds the database. It returns ErrAlreadySeeded when run twice.
func Run(ctx context.Context, s *services.Services) (*Report, error) {
	if !s.Cfg.IsDev() || s.Cfg.Env == config.EnvProduction {
		return nil, errors.New("demo data can only be loaded in development")
	}
	if s.Payment.Name() != "fake" {
		return nil, errors.New("demo data needs PAYMENT_PROVIDER=fake (payments are simulated)")
	}
	if exists, err := s.Repos.Users.Exists(ctx, marker); err != nil {
		return nil, err
	} else if exists {
		return nil, ErrAlreadySeeded
	}
	sd := &seeder{s: s, rnd: rand.New(rand.NewSource(42)), report: &Report{}}
	if err := sd.loadReference(ctx); err != nil {
		return nil, err
	}
	var published []uuid.UUID
	for i, h := range houses {
		ids, err := sd.house(ctx, i, h)
		if err != nil {
			return nil, fmt.Errorf("house %s: %w", h.name, err)
		}
		published = append(published, ids...)
	}
	if err := sd.moderation(ctx, published); err != nil {
		return nil, fmt.Errorf("moderation: %w", err)
	}
	return sd.report, nil
}

type seeder struct {
	s          *services.Services
	rnd        *rand.Rand
	report     *Report
	regions    map[string]uuid.UUID
	categories []uuid.UUID
	nameIdx    int
	paySeq     int
}

func (sd *seeder) loadReference(ctx context.Context) error {
	regs, err := sd.s.Repos.Taxonomy.Regions(ctx, true)
	if err != nil {
		return err
	}
	sd.regions = map[string]uuid.UUID{}
	for _, r := range regs {
		sd.regions[r.Slug] = r.ID
	}
	cats, err := sd.s.Repos.Taxonomy.Categories(ctx, true)
	if err != nil {
		return err
	}
	for _, c := range cats {
		if c.ParentID != nil {
			sd.categories = append(sd.categories, c.ID)
		}
	}
	return nil
}

func as(ctx context.Context, a *rbac.Actor) context.Context { return rbac.WithActor(ctx, a) }

func ptr[T any](v T) *T { return &v }

// house creates a manager, its establishment, its professionals and its
// subscription. It returns the published profile ids.
func (sd *seeder) house(ctx context.Context, idx int, h house) ([]uuid.UUID, error) {
	s := sd.s
	res, err := s.Auth.RegisterManager(ctx, services.RegisterInput{Username: h.username, Password: Password,
		AcceptTerms: true, AcceptCGV: true, AcceptDPA: true})
	if err != nil {
		return nil, err
	}
	_ = s.Auth.ConfirmRecoveryCodes(ctx, &rbac.Actor{UserID: res.User.ID, Role: models.RoleManager})
	est, err := s.Repos.Establishments.ByManager(ctx, res.User.ID)
	if err != nil {
		return nil, err
	}
	mgr := &rbac.Actor{UserID: res.User.ID, Role: models.RoleManager, EstablishmentID: est.ID}
	mctx := as(ctx, mgr)
	hours, _ := json.Marshal(map[string][]map[string]string{
		"mon": {{"open": "12:00", "close": "02:00"}}, "tue": {{"open": "12:00", "close": "02:00"}},
		"wed": {{"open": "12:00", "close": "02:00"}}, "thu": {{"open": "12:00", "close": "03:00"}},
		"fri": {{"open": "14:00", "close": "05:00"}}, "sat": {{"open": "14:00", "close": "05:00"}},
	})
	var address *string
	if h.address != "" {
		address = &h.address
	}
	region, city := sd.regions[h.region], sd.regions[h.city]
	if _, err := s.Establishments.Update(mctx, mgr, services.EstablishmentInput{DisplayName: h.name, LegalName: h.legal,
		Description: h.description, OpeningHours: hours, RegionID: &region, CityID: &city, Address: address,
		AddressPublic: idx%2 == 0}); err != nil {
		return nil, err
	}
	phone := func(n int) string { return fmt.Sprintf("+32 47%d %02d %02d %02d", idx, 10+n, 20+idx, 30+n) }
	if _, err := s.Establishments.ReplaceContacts(mctx, mgr, []services.ContactInput{
		{Type: "PHONE", Value: phone(1), Visibility: models.VisibilityPublic},
		{Type: "WHATSAPP", Value: phone(2), Visibility: models.VisibilityPublic},
		{Type: "PHONE", Value: phone(3), Visibility: models.VisibilityHidden, IsVerificationPhone: true},
		{Type: "WEBSITE", Value: fmt.Sprintf("https://www.%s.example", h.username), Visibility: models.VisibilityPublic},
	}); err != nil {
		return nil, err
	}
	sd.report.Establishments++

	// The subscription comes first: profiles can only be put in service with it.
	if h.subscription != "none" {
		if err := sd.subscribe(mctx, mgr); err != nil {
			return nil, fmt.Errorf("subscription: %w", err)
		}
	}
	var published []uuid.UUID
	for i := 0; i < h.profiles; i++ {
		id, err := sd.professional(ctx, mgr, h, i)
		if err != nil {
			return nil, fmt.Errorf("profile %d: %w", i, err)
		}
		// A profile created by a paid house is in service at once (ADR-021).
		if h.subscription != "none" {
			published = append(published, id)
		}
	}
	// Profiles in preparation: created by the house, no login yet.
	for i := 0; i < h.pending; i++ {
		name := names[sd.nameIdx%len(names)]
		sd.nameIdx++
		if _, err := s.Profiles.CreateByManager(mctx, mgr, services.ProfileInput{DisplayName: name,
			Headline: "Profil en préparation", Languages: []string{"fr"}}, nil); err != nil {
			return nil, fmt.Errorf("draft %d: %w", i, err)
		}
		sd.report.Drafts++
	}
	// Variety for the manager dashboard.
	if h.username == "rose_gerant" && len(published) > 0 {
		if err := s.Profiles.Archive(mctx, mgr, published[len(published)-1]); err != nil {
			return nil, fmt.Errorf("archive: %w", err)
		}
		published = published[:len(published)-1]
		if _, err := s.Profiles.CreateByManager(mctx, mgr, services.ProfileInput{DisplayName: "Profil en préparation",
			Headline: "Brouillon créé par la maison", Languages: []string{"fr"}}, nil); err != nil {
			return nil, fmt.Errorf("draft: %w", err)
		}
	}
	switch h.subscription {
	case "past_due":
		if err := sd.makePastDue(ctx, est.ID); err != nil {
			return nil, err
		}
	case "renewal_due":
		if _, err := s.Billing.Renew(mctx, mgr); err != nil {
			return nil, fmt.Errorf("renewal invoice: %w", err)
		}
	}
	for _, id := range published {
		if err := sd.stats(ctx, id); err != nil {
			return nil, err
		}
	}
	return published, nil
}

// professional is created by the manager with a login (ADR-022), then fills
// its own profile as the professional.
func (sd *seeder) professional(ctx context.Context, mgr *rbac.Actor, h house, i int) (uuid.UUID, error) {
	s := sd.s
	name := names[sd.nameIdx%len(names)]
	sd.nameIdx++
	username := fmt.Sprintf("%s_%s_%d", sluggish(name), h.username[:len(h.username)-len("_gerant")], i)
	d, err := s.Profiles.CreateByManager(as(ctx, mgr), mgr, services.ProfileInput{DisplayName: name},
		&services.AccountInput{Username: username, Password: Password})
	if err != nil {
		return uuid.Nil, err
	}
	p := d.Profile
	pro := &rbac.Actor{UserID: *p.UserID, Role: models.RoleProfessional, EstablishmentID: p.EstablishmentID, ProfileID: p.ID}
	pctx := as(ctx, pro)
	r := sd.rnd
	hair := []string{"BLONDE", "BROWN", "BLACK", "RED"}[r.Intn(4)]
	eyes := []string{"BLUE", "GREEN", "BROWN", "HAZEL"}[r.Intn(4)]
	body := []string{"SLIM", "ATHLETIC", "AVERAGE", "CURVY"}[r.Intn(4)]
	langs := [][]string{{"fr", "en"}, {"fr", "nl", "en"}, {"fr", "es"}, {"nl", "en"}, {"fr", "it", "en"}, {"fr", "ro"}}[r.Intn(6)]
	gender := "FEMALE"
	if name == "Leo" || name == "Andrea" {
		gender = "MALE"
	} else if name == "Sasha" || name == "Kim" {
		gender = []string{"TRANS_WOMAN", "NON_BINARY"}[r.Intn(2)]
	}
	if _, err := s.Profiles.Update(pctx, pro, p.ID, services.ProfileInput{
		DisplayName: name, Headline: headlines[r.Intn(len(headlines))], Description: descriptions[r.Intn(len(descriptions))],
		Age: ptr(21 + r.Intn(24)), HeightCm: ptr(155 + r.Intn(30)), WeightKg: ptr(48 + r.Intn(25)), HairColor: &hair, EyeColor: &eyes, BodyType: &body,
		Languages: langs, ExtraAttributes: map[string]string{"gender": gender, "smoker": []string{"true", "false"}[r.Intn(2)],
			"skin_tone": []string{"FAIR", "LIGHT", "MEDIUM", "OLIVE", "TAN", "BROWN", "DARK"}[r.Intn(7)]},
	}); err != nil {
		return uuid.Nil, err
	}
	base := int64(80+r.Intn(5)*10) * 100
	if _, err := s.Profiles.ReplaceRates(pctx, pro, p.ID, []services.RateInput{
		{Label: "30 minutes", DurationMinutes: ptr(30), AmountCents: base},
		{Label: "1 heure", DurationMinutes: ptr(60), AmountCents: base*2 - 2000},
		{Label: "2 heures", DurationMinutes: ptr(120), AmountCents: base*4 - 6000},
	}); err != nil {
		return uuid.Nil, err
	}
	if _, err := s.Profiles.ReplaceContacts(pctx, pro, p.ID, []services.ContactInput{
		{Type: "WHATSAPP", Value: fmt.Sprintf("+32 48%d %02d %02d %02d", r.Intn(10), r.Intn(90)+10, r.Intn(90)+10, r.Intn(90)+10), Visibility: models.VisibilityPublic},
		{Type: "INSTAGRAM", Value: "https://www.instagram.com/" + username, Visibility: models.VisibilityPublic},
	}); err != nil {
		return uuid.Nil, err
	}
	if len(sd.categories) > 0 {
		r.Shuffle(len(sd.categories), func(a, b int) { sd.categories[a], sd.categories[b] = sd.categories[b], sd.categories[a] })
		if _, err := s.Profiles.ReplaceCategories(pctx, pro, p.ID, sd.categories[:2+r.Intn(3)]); err != nil {
			return uuid.Nil, err
		}
	}
	for k := 0; k < 2+r.Intn(3); k++ {
		if _, err := s.Profiles.UploadPhoto(pctx, pro, p.ID, portrait(r, k)); err != nil {
			return uuid.Nil, fmt.Errorf("photo: %w", err)
		}
	}
	sd.report.Profiles++
	return p.ID, nil
}

func sluggish(name string) string {
	out := []rune{}
	for _, r := range name {
		switch r {
		case 'é', 'è', 'ê':
			r = 'e'
		case 'à':
			r = 'a'
		case 'ï', 'î':
			r = 'i'
		case 'ç':
			r = 'c'
		}
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		if r >= 'a' && r <= 'z' {
			out = append(out, r)
		}
	}
	return string(out)
}

// subscribe creates the subscription and confirms the payment with a signed
// IPN of the fake provider (same path as a real payment).
func (sd *seeder) subscribe(ctx context.Context, mgr *rbac.Actor) error {
	pay, err := sd.s.Billing.Subscribe(ctx, mgr, "")
	if err != nil {
		return err
	}
	return sd.confirm(ctx, pay)
}

func (sd *seeder) confirm(ctx context.Context, pay *models.Payment) error {
	sd.paySeq++
	invoiceID := ""
	if pay.ProviderInvoiceID != nil {
		invoiceID = *pay.ProviderInvoiceID
	}
	body, _ := json.Marshal(map[string]any{
		"payment_id": json.Number(fmt.Sprintf("%d", 900000+sd.paySeq+int(time.Now().Unix()%100000)*10)), "payment_status": "finished",
		"order_id": pay.ID.String(), "invoice_id": invoiceID, "price_amount": json.Number(payment.CentsToDecimal(pay.AmountCents)),
		"price_currency": "eur", "pay_amount": json.Number("0.0061"), "actually_paid": json.Number("0.0061"), "pay_currency": "btc",
	})
	sig, err := payment.SignIPN([]byte(sd.s.Cfg.NOWPaymentsIPNSecret), body)
	if err != nil {
		return err
	}
	return sd.s.Billing.HandleIPN(ctx, body, sig)
}

// makePastDue moves an active subscription to PAST_DUE within its grace period.
func (sd *seeder) makePastDue(ctx context.Context, establishmentID uuid.UUID) error {
	sub, err := sd.s.Repos.Billing.CurrentSubscription(ctx, establishmentID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := sd.s.Repos.Billing.UpdateSubscription(ctx, sub.ID, map[string]any{"status": models.SubPastDue,
		"current_period_start": now.AddDate(0, 0, -31), "current_period_end": now.Add(-24 * time.Hour),
		"grace_until": now.Add(48 * time.Hour)}); err != nil {
		return err
	}
	return sd.s.Publication.Recompute(ctx, establishmentID)
}

// stats writes 30 days of aggregated views for a published profile.
func (sd *seeder) stats(ctx context.Context, profileID uuid.UUID) error {
	now := time.Now().UTC()
	weight := 5 + sd.rnd.Intn(40)
	for d := 0; d < 30; d++ {
		views := int64(weight + sd.rnd.Intn(weight+1))
		if wd := now.AddDate(0, 0, -d).Weekday(); wd == time.Friday || wd == time.Saturday {
			views = views * 3 / 2
		}
		if err := sd.s.Stats.SeedCounts(ctx, profileID, now.AddDate(0, 0, -d), views, views/(4+int64(sd.rnd.Intn(4)))); err != nil {
			return err
		}
	}
	return nil
}

// moderation adds an admin action (hidden profile).
func (sd *seeder) moderation(ctx context.Context, published []uuid.UUID) error {
	s := sd.s
	if len(published) > 3 {
		if err := s.Admin.HideProfile(ctx, published[3], true, "Photo non conforme — en attente d’une nouvelle photo"); err != nil {
			return err
		}
	}
	return nil
}

// portrait draws an abstract silhouette (no real person) as a JPEG.
func portrait(r *rand.Rand, variant int) []byte {
	const w, h = 720, 960
	bg := palette[r.Intn(len(palette))]
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	shade := func(c color.RGBA, f float64) color.RGBA {
		return color.RGBA{uint8(float64(c.R) * f), uint8(float64(c.G) * f), uint8(float64(c.B) * f), 255}
	}
	light := shade(bg, 1.25)
	dark := shade(bg, 0.45)
	cx := 360 + (variant%3-1)*60
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := bg
			// Soft vertical bands evoke a room backdrop.
			if (x/90+variant)%2 == 0 {
				c = shade(bg, 0.92)
			}
			// A warm light disc behind the silhouette.
			if dx, dy := x-cx-140, y-260; dx*dx+dy*dy < 180*180 {
				c = light
			}
			// Head and shoulders.
			if dx, dy := x-cx, y-360; dx*dx*100/(105*105)+dy*dy*100/(130*130) < 100 {
				c = dark
			}
			if y > 520 {
				if dx, dy := x-cx, y-900; dx*dx*100/(260*260)+dy*dy*100/(400*400) < 100 {
					c = dark
				}
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	return buf.Bytes()
}
