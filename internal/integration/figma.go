package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"storr-api/internal/httpclient"
	"storr-api/internal/secrets"
	"storr-api/internal/store"

	"github.com/google/uuid"
)

// FigmaOAuthConfig holds the client credentials for the Figma OAuth flow.
type FigmaOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// FigmaOAuthService manages the Figma OAuth lifecycle including token exchange and refresh.
type FigmaOAuthService struct {
	config     FigmaOAuthConfig
	stateStore *OAuthStateStore
	secrets    *secrets.SecretsClient
	stores     *store.Service
}

// NewFigmaOAuthService creates a FigmaOAuthService with the given dependencies.
func NewFigmaOAuthService(config FigmaOAuthConfig, stateStore *OAuthStateStore, secrets *secrets.SecretsClient, stores *store.Service) *FigmaOAuthService {
	return &FigmaOAuthService{
		config:     config,
		stateStore: stateStore,
		secrets:    secrets,
		stores:     stores,
	}
}

// Start generates a Figma OAuth authorization URL and persists the CSRF state token.
func (f *FigmaOAuthService) Start(ctx context.Context, customerID, storeID string) (*StartFigmaResponse, error) {
	state := uuid.NewString()
	if err := f.stateStore.Put(ctx, OAuthState{
		State:      state,
		CustomerID: customerID,
		Provider:   store.ProviderFigma,
		StoreID:    storeID,
	}); err != nil {
		return nil, fmt.Errorf("store oauth state: %w", err)
	}
	params := url.Values{
		"client_id":     {f.config.ClientID},
		"redirect_uri":  {f.config.RedirectURI},
		"scope":         {FigmaOAuthScopes},
		"state":         {state},
		"response_type": {"code"},
	}
	authURL := FigmaOAuthBaseURL + params.Encode()
	return &StartFigmaResponse{AuthURL: authURL, State: state}, nil
}

// CompleteWithCode exchanges the authorization code for tokens and saves the integration record.
func (f *FigmaOAuthService) CompleteWithCode(ctx context.Context, customerID, storeID, code string) error {
	tokens, err := f.exchangeCode(ctx, code)
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}
	if _, err := f.storeFigmaSecret(ctx, customerID, storeID, tokens); err != nil {
		return err
	}
	return f.stores.PutIntegration(ctx, customerID, storeID, store.IntegrationUpdate{
		Key:    store.ProviderFigma,
		Config: store.IntegrationConfig{Enabled: true, Origin: IntegrationOriginOAuth},
	})
}

func (f *FigmaOAuthService) storeFigmaSecret(ctx context.Context, customerID, storeID string, tokens *secrets.FigmaSecret) (string, error) {
	secretName := fmt.Sprintf(secrets.FigmaSecretNameFmt, customerID, storeID)
	arn, err := secrets.PutSecret(f.secrets, ctx, secretName, *tokens)
	if err != nil {
		return "", fmt.Errorf("store secret: %w", err)
	}
	return arn, nil
}

func (f *FigmaOAuthService) exchangeCode(ctx context.Context, code string) (*secrets.FigmaSecret, error) {
	req, err := f.buildTokenRequest(ctx, code)
	if err != nil {
		return nil, err
	}
	return parseFigmaTokenResponse(httpclient.Default.Do(req))
}

func (f *FigmaOAuthService) buildTokenRequest(ctx context.Context, code string) (*http.Request, error) {
	data := url.Values{
		"client_id":     {f.config.ClientID},
		"client_secret": {f.config.ClientSecret},
		"redirect_uri":  {f.config.RedirectURI},
		"code":          {code},
		"grant_type":    {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, FigmaTokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

type figmaTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func parseFigmaTokenResponse(resp *http.Response, err error) (*secrets.FigmaSecret, error) {
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("figma token exchange failed: status %d", resp.StatusCode)
	}
	var tokenResp figmaTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	return &secrets.FigmaSecret{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    expiresAt,
	}, nil
}

// RefreshIfNeeded proactively refreshes the Figma token if it expires within the threshold window.
func (f *FigmaOAuthService) RefreshIfNeeded(ctx context.Context, secret *secrets.FigmaSecret) (*secrets.FigmaSecret, error) {
	expiresAt, err := time.Parse(time.RFC3339, secret.ExpiresAt)
	if err != nil {
		return secret, nil
	}
	if time.Until(expiresAt) > FigmaTokenRefreshThreshold {
		return secret, nil
	}
	return f.refreshToken(ctx, secret.RefreshToken)
}

func (f *FigmaOAuthService) refreshToken(ctx context.Context, refreshToken string) (*secrets.FigmaSecret, error) {
	req, err := f.buildRefreshRequest(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	secret, err := parseFigmaRefreshResponse(httpclient.Default.Do(req))
	if err != nil {
		return nil, err
	}
	secret.RefreshToken = refreshToken
	return secret, nil
}

func (f *FigmaOAuthService) buildRefreshRequest(ctx context.Context, refreshToken string) (*http.Request, error) {
	data := url.Values{
		"client_id":     {f.config.ClientID},
		"client_secret": {f.config.ClientSecret},
		"refresh_token": {refreshToken},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, FigmaRefreshEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

type figmaRefreshResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

func parseFigmaRefreshResponse(resp *http.Response, err error) (*secrets.FigmaSecret, error) {
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("figma token refresh failed: status %d", resp.StatusCode)
	}
	var tokenResp figmaRefreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}
	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	return &secrets.FigmaSecret{
		AccessToken: tokenResp.AccessToken,
		ExpiresAt:   expiresAt,
	}, nil
}
