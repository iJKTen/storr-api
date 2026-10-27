package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/aws/aws-sdk-go-v2/aws"
	cognito "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	cognitotypes "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/golang-jwt/jwt/v5"
)

type cognitoClaims struct {
	jwt.RegisteredClaims
	ClientID string `json:"client_id"`
	TokenUse string `json:"token_use"`
	Email    string `json:"email"`
	Name     string `json:"name"`
}

// Service handles user authentication against AWS Cognito.
type Service struct {
	client   *cognito.Client
	poolID   string
	clientID string
	jwks     keyfunc.Keyfunc
	issuer   string
}

// NewService initializes the auth service with Cognito credentials and JWKS verification.
func NewService(client *cognito.Client, poolID, clientID string) *Service {
	region := strings.Split(poolID, "_")[0]
	issuer := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, poolID)
	jwksURL := fmt.Sprintf("%s/.well-known/jwks.json", issuer)

	k, err := keyfunc.NewDefault([]string{jwksURL})
	if err != nil {
		slog.Error("failed to fetch Cognito JWKS", "error", err)
		os.Exit(1)
	}

	return &Service{
		client:   client,
		poolID:   poolID,
		clientID: clientID,
		jwks:     k,
		issuer:   issuer,
	}
}

// VerifyToken validates a JWT and returns the associated user.
// Accepts both ID tokens (which contain email/name) and access tokens (which don't).
// For access tokens, falls back to GetMe to populate the full user profile.
func (s *Service) VerifyToken(ctx context.Context, tokenString string) (*User, error) {
	claims, err := s.parseToken(tokenString)
	if err != nil {
		return nil, err
	}
	if err := s.validateClaims(claims); err != nil {
		return nil, err
	}
	user := extractUser(claims)
	if user.ID == "" || user.Email == "" {
		return s.GetMe(ctx, tokenString)
	}
	return user, nil
}

func (s *Service) parseToken(tokenString string) (*cognitoClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &cognitoClaims{}, s.jwks.Keyfunc)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("token is not valid")
	}
	claims, ok := token.Claims.(*cognitoClaims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}

func (s *Service) validateClaims(claims *cognitoClaims) error {
	if claims.Issuer != s.issuer {
		return fmt.Errorf("invalid issuer: %s", claims.Issuer)
	}
	if claims.TokenUse != "id" && claims.TokenUse != "access" {
		return fmt.Errorf("invalid token_use: %s", claims.TokenUse)
	}
	audMatch := false
	for _, a := range claims.Audience {
		if a == s.clientID {
			audMatch = true
			break
		}
	}
	if !audMatch && claims.ClientID != s.clientID {
		return errors.New("invalid audience/client_id")
	}
	return nil
}

func extractUser(claims *cognitoClaims) *User {
	return &User{
		ID:          claims.Subject,
		Email:       claims.Email,
		DisplayName: claims.Name,
	}
}

// SignUp creates a new Cognito user, auto-confirms them, and returns authentication tokens.
func (s *Service) SignUp(ctx context.Context, email, password, displayName string) (*AuthResponse, error) {
	_, err := s.client.SignUp(ctx, &cognito.SignUpInput{
		ClientId: aws.String(s.clientID),
		Username: aws.String(email),
		Password: aws.String(password),
		UserAttributes: []cognitotypes.AttributeType{
			{Name: aws.String("email"), Value: aws.String(email)},
			{Name: aws.String("name"), Value: aws.String(displayName)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cognito SignUp: %w", err)
	}
	if err := s.confirmUser(ctx, email); err != nil {
		return nil, err
	}
	return s.authenticate(ctx, email, password)
}

func (s *Service) confirmUser(ctx context.Context, email string) error {
	_, err := s.client.AdminConfirmSignUp(ctx, &cognito.AdminConfirmSignUpInput{
		UserPoolId: aws.String(s.poolID),
		Username:   aws.String(email),
	})
	if err != nil {
		return fmt.Errorf("cognito AdminConfirmSignUp: %w", err)
	}
	return nil
}

// Login authenticates against Cognito and returns tokens.
func (s *Service) Login(ctx context.Context, email, password string) (*AuthResponse, error) {
	return s.authenticate(ctx, email, password)
}

// GetMe fetches the user's profile from Cognito using an access token.
func (s *Service) GetMe(ctx context.Context, accessToken string) (*User, error) {
	out, err := s.client.GetUser(ctx, &cognito.GetUserInput{
		AccessToken: aws.String(accessToken),
	})
	if err != nil {
		return nil, err
	}

	user := User{ID: *out.Username}
	for _, attr := range out.UserAttributes {
		switch *attr.Name {
		case "sub":
			user.ID = *attr.Value
		case "email":
			user.Email = *attr.Value
		case "name":
			user.DisplayName = *attr.Value
		}
	}
	return &user, nil
}

// RefreshToken exchanges a refresh token for a new access token via Cognito.
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*AuthTokens, error) {
	out, err := s.client.AdminInitiateAuth(ctx, &cognito.AdminInitiateAuthInput{
		UserPoolId: aws.String(s.poolID),
		ClientId:   aws.String(s.clientID),
		AuthFlow:   cognitotypes.AuthFlowTypeRefreshTokenAuth,
		AuthParameters: map[string]string{
			"REFRESH_TOKEN": refreshToken,
		},
	})
	if err != nil {
		return nil, err
	}
	if out.AuthenticationResult == nil {
		return nil, fmt.Errorf("refresh failed")
	}
	return &AuthTokens{
		AccessToken: *out.AuthenticationResult.AccessToken,
		ExpiresIn:   out.AuthenticationResult.ExpiresIn,
	}, nil
}

func (s *Service) authenticate(ctx context.Context, email, password string) (*AuthResponse, error) {
	result, err := s.initiateAuth(ctx, email, password)
	if err != nil {
		return nil, err
	}
	return s.buildAuthResponse(ctx, result)
}

func (s *Service) initiateAuth(ctx context.Context, email, password string) (*cognitotypes.AuthenticationResultType, error) {
	out, err := s.client.AdminInitiateAuth(ctx, &cognito.AdminInitiateAuthInput{
		UserPoolId: aws.String(s.poolID),
		ClientId:   aws.String(s.clientID),
		AuthFlow:   cognitotypes.AuthFlowTypeAdminUserPasswordAuth,
		AuthParameters: map[string]string{
			"USERNAME": email,
			"PASSWORD": password,
		},
	})
	if err != nil {
		return nil, err
	}
	if out.AuthenticationResult == nil {
		return nil, fmt.Errorf("authentication challenge not supported")
	}
	return out.AuthenticationResult, nil
}

func (s *Service) buildAuthResponse(ctx context.Context, result *cognitotypes.AuthenticationResultType) (*AuthResponse, error) {
	user, err := s.VerifyToken(ctx, *result.IdToken)
	if err != nil {
		return nil, fmt.Errorf("verify id token: %w", err)
	}
	return &AuthResponse{
		User: *user,
		Tokens: &AuthTokens{
			AccessToken:  *result.AccessToken,
			RefreshToken: *result.RefreshToken,
			ExpiresIn:    result.ExpiresIn,
		},
	}, nil
}
