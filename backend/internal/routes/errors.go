package routes

import "opensex/backend/internal/apperr"

func apperrNotFound() error { return apperr.NotFound("resource") }
