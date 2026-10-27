package auth

// MinPasswordLength is the minimum number of characters required for user passwords.
const MinPasswordLength = 8

// RefreshTokenMaxAge is the max-age (in seconds) for the refresh token cookie.
// Equals 30 days.
const RefreshTokenMaxAge = 30 * 24 * 3600

// CookieAccessToken is the HTTP cookie name for the access token.
const CookieAccessToken = "accessToken"

// CookieRefreshToken is the HTTP cookie name for the refresh token.
const CookieRefreshToken = "refreshToken"

// EmailPattern is the regex pattern for validating email addresses.
// Case-insensitive; supports TLDs of any length (e.g. .store, .digital).
const EmailPattern = `(?i)^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`
