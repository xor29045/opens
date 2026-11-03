// Package repositories contains the parameterised GORM queries, one type per
// aggregate. No business rule lives here; the caller's transaction (if any)
// is taken from the context.
package repositories

import (
	"context"

	"gorm.io/gorm"

	"opensex/backend/internal/database"
)

type base struct{ DB *database.DB }

func (b base) db(ctx context.Context) *gorm.DB { return b.DB.Conn(ctx) }

type Repos struct {
	Users          *UserRepo
	Tokens         *TokenRepo
	Establishments *EstablishmentRepo
	Profiles       *ProfileRepo
	Taxonomy       *TaxonomyRepo
	Favorites      *FavoriteRepo
	Billing        *BillingRepo
	Catalog        *CatalogRepo
}

func New(db *database.DB) *Repos {
	b := base{DB: db}
	return &Repos{
		Users:          &UserRepo{b},
		Tokens:         &TokenRepo{b},
		Establishments: &EstablishmentRepo{b},
		Profiles:       &ProfileRepo{b},
		Taxonomy:       &TaxonomyRepo{b},
		Favorites:      &FavoriteRepo{b},
		Billing:        &BillingRepo{b},
		Catalog:        &CatalogRepo{b},
	}
}

// Page describes a requested page.
type Page struct {
	Page     int
	PageSize int
}

func (p Page) Offset() int { return (p.Page - 1) * p.PageSize }

// NormalizePage applies defaults and the 50 items maximum (§11).
func NormalizePage(page, size int) Page {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 50 {
		size = 50
	}
	return Page{Page: page, PageSize: size}
}
