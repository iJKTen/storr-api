package auth

import (
	"net/http"

	"storr-api/internal/api"
)

// RequireAuth returns a Chi middleware that rejects unauthenticated requests.
func RequireAuth(authService *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := ExtractBearerToken(r)
			if token == "" {
				api.WriteError(w, http.StatusUnauthorized, "Missing authorization", "UNAUTHORIZED")
				return
			}

			user, err := authService.VerifyToken(r.Context(), token)
			if err != nil {
				api.WriteError(w, http.StatusUnauthorized, "Invalid or expired token", "UNAUTHORIZED")
				return
			}

			ctx := WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
