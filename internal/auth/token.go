package auth

import (
	"net/http"
	"strings"
)

// ExtractBearerToken retrieves the token from the Authorization header or the accessToken cookie.
func ExtractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return authHeader[7:]
	}

	// Fallback to cookie
	if cookie, err := r.Cookie("accessToken"); err == nil {
		return cookie.Value
	}

	return ""
}
