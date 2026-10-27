package auth

// SignupRequest is the expected JSON body for the signup endpoint.
type SignupRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

// LoginRequest is the expected JSON body for the login endpoint.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// User represents an authenticated user's profile.
type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

// AuthTokens holds the JWT tokens issued by Cognito after authentication.
type AuthTokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int32  `json:"expiresIn"`
}

// AuthResponse is returned to the client after successful signup or login.
type AuthResponse struct {
	User   User        `json:"user"`
	Tokens *AuthTokens `json:"tokens,omitempty"`
}

