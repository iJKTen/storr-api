package auth

import (
	"context"
	"net/http"
)

type contextKey string

const userContextKey contextKey = "user"

// WithUser stores the authenticated user in the request context for downstream handlers.
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext retrieves the authenticated user set by the auth middleware.
func UserFromContext(ctx context.Context) *User {
	user, _ := ctx.Value(userContextKey).(*User)
	return user
}

// UserFromRequest is a convenience wrapper that extracts the user from the request's context.
func UserFromRequest(r *http.Request) *User {
	return UserFromContext(r.Context())
}
