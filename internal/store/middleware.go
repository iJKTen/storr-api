package store

import (
	"errors"
	"net/http"

	"storr-api/internal/api"
	"storr-api/internal/auth"

	"github.com/go-chi/chi/v5"
)

// RequireStoreOwnership returns a Chi middleware that verifies the authenticated
// user owns the store identified by the {id} URL parameter. Returns 404 if the
// store does not exist or does not belong to the user, preventing IDOR attacks.
func RequireStoreOwnership(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := auth.UserFromRequest(r)
			if user == nil {
				api.WriteError(w, http.StatusUnauthorized, "Missing authorization", "UNAUTHORIZED")
				return
			}
			storeID := chi.URLParam(r, "id")
			if _, err := svc.Get(r.Context(), user.ID, storeID); err != nil {
				if errors.Is(err, ErrStoreNotFound) {
					api.WriteError(w, http.StatusNotFound, "Store not found", "NOT_FOUND")
					return
				}
				api.WriteInternalError(w, "Failed to verify store ownership", err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
