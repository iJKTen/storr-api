package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"storr-api/internal/api"

	cognitotypes "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

var emailRegex = regexp.MustCompile(EmailPattern)

// Handler provides HTTP endpoints for user authentication and session management.
type Handler struct {
	service *Service
}

// NewHandler creates an auth Handler backed by the given authentication service.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

// SignUp registers a new user account and returns authentication tokens.
func (h *Handler) SignUp(w http.ResponseWriter, r *http.Request) {
	req, err := decodeSignupRequest(r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body", "VALIDATION_ERROR")
		return
	}
	if details := validateSignupRequest(req); len(details) > 0 {
		api.WriteValidationError(w, details)
		return
	}
	resp, err := h.service.SignUp(r.Context(), req.Email, req.Password, deriveDisplayName(req.Email))
	if err != nil {
		slog.Error("signup failed", "email", req.Email, "error", err)
		status, msg, code := mapCognitoError(err)
		api.WriteError(w, status, msg, code)
		return
	}
	writeAuthResponse(w, resp)
}

// Login authenticates a user with email and password and returns tokens.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	req, err := decodeLoginRequest(r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body", "VALIDATION_ERROR")
		return
	}
	if details := validateLoginRequest(req); len(details) > 0 {
		api.WriteValidationError(w, details)
		return
	}
	resp, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		slog.Error("login failed", "email", req.Email, "error", err)
		status, msg, code := mapCognitoError(err)
		api.WriteError(w, status, msg, code)
		return
	}
	writeAuthResponse(w, resp)
}

// Refresh exchanges a valid refresh token cookie for new access tokens.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(CookieRefreshToken)
	if err != nil {
		api.WriteError(w, http.StatusUnauthorized, "Missing refresh token", "UNAUTHORIZED")
		return
	}

	tokens, err := h.service.RefreshToken(r.Context(), cookie.Value)
	if err != nil {
		slog.Error("refresh failed", "error", err)
		api.WriteError(w, http.StatusUnauthorized, "Invalid or expired refresh token", "UNAUTHORIZED")
		return
	}

	setAuthCookies(w, tokens)
	api.WriteStatusOK(w)
}

// Logout clears authentication cookies to end the user's session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, newSecureCookie(CookieAccessToken, "", -1))
	http.SetCookie(w, newSecureCookie(CookieRefreshToken, "", -1))
	api.WriteStatusOK(w)
}

// Me returns the authenticated user's profile from the access token.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	token := ExtractBearerToken(r)
	if token == "" {
		api.WriteError(w, http.StatusUnauthorized, "Missing authorization header", "UNAUTHORIZED")
		return
	}

	user, err := h.service.VerifyToken(r.Context(), token)
	if err != nil {
		api.WriteError(w, http.StatusUnauthorized, "Invalid or expired token", "UNAUTHORIZED")
		return
	}

	// Access tokens lack email/name claims; fetch full profile from Cognito.
	if user.Email == "" {
		user, err = h.service.GetMe(r.Context(), token)
		if err != nil {
			api.WriteError(w, http.StatusInternalServerError, "Failed to fetch user profile", "INTERNAL_ERROR")
			return
		}
	}

	data, err := json.Marshal(user)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

func writeAuthResponse(w http.ResponseWriter, resp *AuthResponse) {
	setAuthCookies(w, resp.Tokens)
	resp.Tokens = nil
	data, err := json.Marshal(resp)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

func validateSignupRequest(req *SignupRequest) map[string]string {
	details := make(map[string]string)
	if req.Email == "" {
		details["email"] = "Email is required"
	} else if !emailRegex.MatchString(req.Email) {
		details["email"] = "Invalid email format"
	}
	if req.Password == "" {
		details["password"] = "Password is required"
	} else if len(req.Password) < MinPasswordLength {
		details["password"] = "Password must be at least 8 characters"
	}
	if req.Password != req.ConfirmPassword {
		details["confirmPassword"] = "Passwords do not match"
	}
	return details
}

func validateLoginRequest(req *LoginRequest) map[string]string {
	details := make(map[string]string)
	if req.Email == "" {
		details["email"] = "Email is required"
	}
	if req.Password == "" {
		details["password"] = "Password is required"
	}
	return details
}

func deriveDisplayName(email string) string {
	if idx := strings.Index(email, "@"); idx != -1 {
		return email[:idx]
	}
	return email
}

func setAuthCookies(w http.ResponseWriter, tokens *AuthTokens) {
	if tokens.AccessToken != "" {
		http.SetCookie(w, newSecureCookie(CookieAccessToken, tokens.AccessToken, int(tokens.ExpiresIn)))
	}
	if tokens.RefreshToken != "" {
		http.SetCookie(w, newSecureCookie(CookieRefreshToken, tokens.RefreshToken, RefreshTokenMaxAge))
	}
}

func newSecureCookie(name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		MaxAge:   maxAge,
	}
}

func decodeSignupRequest(r *http.Request) (*SignupRequest, error) {
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	return &req, nil
}

func decodeLoginRequest(r *http.Request) (*LoginRequest, error) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	return &req, nil
}

type cognitoErrorMapping struct {
	match  func(error) bool
	status int
	msg    string
	code   string
}

func asMatch[T error]() func(error) bool {
	return func(err error) bool {
		var t T
		return errors.As(err, &t)
	}
}

var cognitoErrorMappings = []cognitoErrorMapping{
	{match: asMatch[*cognitotypes.UsernameExistsException](), status: http.StatusConflict, msg: "An account with this email already exists", code: "EMAIL_EXISTS"},
	{match: asMatch[*cognitotypes.NotAuthorizedException](), status: http.StatusUnauthorized, msg: "Invalid email or password", code: "INVALID_CREDENTIALS"},
	{match: asMatch[*cognitotypes.UserNotFoundException](), status: http.StatusUnauthorized, msg: "Invalid email or password", code: "INVALID_CREDENTIALS"},
	{match: asMatch[*cognitotypes.InvalidPasswordException](), status: http.StatusBadRequest, msg: "Password does not meet requirements", code: "INVALID_PASSWORD"},
	{match: asMatch[*cognitotypes.InvalidParameterException](), status: http.StatusBadRequest, msg: "Invalid request parameters", code: "INVALID_PARAMETERS"},
}

func mapCognitoError(err error) (int, string, string) {
	for _, m := range cognitoErrorMappings {
		if m.match(err) {
			return m.status, m.msg, m.code
		}
	}
	return http.StatusInternalServerError, "An unexpected error occurred", "INTERNAL_ERROR"
}
