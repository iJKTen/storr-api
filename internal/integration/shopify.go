package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"storr-api/internal/httpclient"
	"storr-api/internal/secrets"
	"storr-api/internal/store"

	"github.com/google/uuid"
)

var shopifyDomainRegex = regexp.MustCompile(ShopifyDomainPattern)

// ValidateShopDomain checks that a shop value is a legitimate myshopify.com subdomain.
// The input should be the bare domain (e.g. "my-store.myshopify.com"), not a full URL.
func ValidateShopDomain(shop string) error {
	// Strip https:// prefix if present so callers can pass either form.
	domain := strings.TrimPrefix(shop, "https://")
	if !shopifyDomainRegex.MatchString(domain) {
		return fmt.Errorf("invalid Shopify domain: must be a .myshopify.com subdomain")
	}
	return nil
}

// ShopifyOAuthConfig holds the client credentials for the Shopify OAuth flow.
type ShopifyOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// ShopifyOAuthService manages the Shopify OAuth lifecycle including token exchange and storage.
type ShopifyOAuthService struct {
	config     ShopifyOAuthConfig
	stateStore *OAuthStateStore
	secrets    *secrets.SecretsClient
	stores     *store.Service
}

// NewShopifyOAuthService creates a ShopifyOAuthService with the given dependencies.
func NewShopifyOAuthService(config ShopifyOAuthConfig, stateStore *OAuthStateStore, secrets *secrets.SecretsClient, stores *store.Service) *ShopifyOAuthService {
	return &ShopifyOAuthService{
		config:     config,
		stateStore: stateStore,
		secrets:    secrets,
		stores:     stores,
	}
}

// Start generates a Shopify OAuth install URL and persists the CSRF state token.
func (s *ShopifyOAuthService) Start(ctx context.Context, customerID, storeID, shop string) (*StartShopifyResponse, error) {
	if err := ValidateShopDomain(shop); err != nil {
		return nil, err
	}
	nonce := uuid.NewString()
	if err := s.stateStore.Put(ctx, OAuthState{
		State:      nonce,
		CustomerID: customerID,
		Provider:   store.ProviderShopify,
		StoreID:    storeID,
		Shop:       shop,
	}); err != nil {
		return nil, fmt.Errorf("store oauth state: %w", err)
	}
	params := url.Values{
		"client_id":    {s.config.ClientID},
		"scope":        {ShopifyOAuthScopes},
		"redirect_uri": {s.config.RedirectURI},
		"state":        {nonce},
	}
	installURL := fmt.Sprintf("%s/admin/oauth/authorize?%s", shop, params.Encode())
	return &StartShopifyResponse{InstallURL: installURL, State: nonce}, nil
}

// ShopifyOAuthCompletion groups the parameters needed to complete a Shopify OAuth flow.
type ShopifyOAuthCompletion struct {
	CustomerID string
	StoreID    string
	Shop       string
	Code       string
}

// CompleteWithCode exchanges the authorization code for tokens and saves the integration record.
func (s *ShopifyOAuthService) CompleteWithCode(ctx context.Context, params ShopifyOAuthCompletion) error {
	if err := s.exchangeAndStoreTokens(ctx, shopifyTokenExchangeParams{
		Shop:       params.Shop,
		Code:       params.Code,
		CustomerID: params.CustomerID,
		ProjectID:  params.StoreID,
	}); err != nil {
		return err
	}
	return s.saveIntegrationRecord(ctx, params.CustomerID, params.StoreID)
}

type shopifyTokenExchangeParams struct {
	Shop       string
	Code       string
	CustomerID string
	ProjectID  string
}

func (s *ShopifyOAuthService) exchangeAndStoreTokens(ctx context.Context, p shopifyTokenExchangeParams) error {
	adminToken, err := s.exchangeCode(ctx, p.Shop, p.Code)
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}
	storefrontToken, err := s.createStorefrontToken(ctx, p.Shop, adminToken)
	if err != nil {
		return fmt.Errorf("create storefront token: %w", err)
	}
	secretName := fmt.Sprintf(secrets.ShopifySecretNameFmt, p.CustomerID, p.ProjectID)
	_, err = secrets.PutSecret(s.secrets, ctx, secretName, secrets.ShopifySecret{
		AdminToken:      adminToken,
		StorefrontToken: storefrontToken,
		StoreDomain:     p.Shop,
		APIVersion:      secrets.ShopifyAPIVersion,
	})
	if err != nil {
		return fmt.Errorf("store secret: %w", err)
	}
	return nil
}

func (s *ShopifyOAuthService) saveIntegrationRecord(ctx context.Context, customerID, projectID string) error {
	return s.stores.PutIntegration(ctx, customerID, projectID, store.IntegrationUpdate{
		Key:    store.ProviderShopify,
		Config: store.IntegrationConfig{Enabled: true, Origin: IntegrationOriginOAuth},
	})
}

// ValidateHMAC validates the HMAC signature from Shopify callback query params.
// Per Shopify spec, validation covers all query params except hmac itself.
func (s *ShopifyOAuthService) ValidateHMAC(queryParams url.Values) bool {
	params := make(map[string]string)
	for key, values := range queryParams {
		if key == "hmac" || key == "signature" {
			continue
		}
		params[key] = values[0]
	}
	expectedHMAC := queryParams.Get("hmac")
	if expectedHMAC == "" {
		return false
	}
	message := buildHMACMessage(params)
	mac := hmac.New(sha256.New, []byte(s.config.ClientSecret))
	mac.Write([]byte(message))
	computed := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(computed), []byte(expectedHMAC))
}

func buildHMACMessage(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}
	return strings.Join(parts, "&")
}

func (s *ShopifyOAuthService) exchangeCode(ctx context.Context, shop, code string) (string, error) {
	req, err := s.buildTokenRequest(ctx, shop, code)
	if err != nil {
		return "", err
	}
	return s.parseTokenResponse(req)
}

func (s *ShopifyOAuthService) buildTokenRequest(ctx context.Context, shop, code string) (*http.Request, error) {
	body, err := json.Marshal(map[string]string{
		"client_id":     s.config.ClientID,
		"client_secret": s.config.ClientSecret,
		"code":          code,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal token request body: %w", err)
	}
	reqURL := fmt.Sprintf("%s/admin/oauth/access_token", shop)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (s *ShopifyOAuthService) parseTokenResponse(req *http.Request) (string, error) {
	resp, err := httpclient.Default.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("shopify token exchange failed: status %d", resp.StatusCode)
	}
	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}
	return tokenResp.AccessToken, nil
}

func (s *ShopifyOAuthService) createStorefrontToken(ctx context.Context, shop, adminToken string) (string, error) {
	req, err := s.buildGQLRequest(ctx, shop, adminToken)
	if err != nil {
		return "", err
	}
	return s.parseGQLResponse(req)
}

type gqlError struct {
	Message string `json:"message"`
}

type storefrontGQLResponse struct {
	Errors []gqlError `json:"errors"`
	Data   struct {
		StorefrontAccessTokenCreate struct {
			StorefrontAccessToken struct {
				AccessToken string `json:"accessToken"`
			} `json:"storefrontAccessToken"`
			UserErrors []struct {
				Field   []string `json:"field"`
				Message string   `json:"message"`
			} `json:"userErrors"`
		} `json:"storefrontAccessTokenCreate"`
	} `json:"data"`
}

func (s *ShopifyOAuthService) buildGQLRequest(ctx context.Context, shop, adminToken string) (*http.Request, error) {
	query := fmt.Sprintf(`mutation {
		storefrontAccessTokenCreate(input: { title: "%s" }) {
			storefrontAccessToken { accessToken }
			userErrors { field message }
		}
	}`, ShopifyStorefrontTokenTitle)
	body, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return nil, fmt.Errorf("marshal graphql request body: %w", err)
	}
	reqURL := fmt.Sprintf("%s/admin/api/%s/graphql.json", shop, secrets.ShopifyAPIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Access-Token", adminToken)
	return req, nil
}

// parseGQLResponse orchestrates executing a GraphQL request and extracting the storefront token.
func (s *ShopifyOAuthService) parseGQLResponse(req *http.Request) (string, error) {
	gqlResp, err := executeGQLRequest(req)
	if err != nil {
		return "", err
	}
	return extractStorefrontToken(gqlResp)
}

// executeGQLRequest sends an HTTP request and decodes the JSON body into a storefrontGQLResponse.
func executeGQLRequest(req *http.Request) (*storefrontGQLResponse, error) {
	resp, err := httpclient.Default.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("storefront token creation failed: status %d", resp.StatusCode)
	}
	var gqlResp storefrontGQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return nil, fmt.Errorf("decode graphql response: %w", err)
	}
	return &gqlResp, nil
}

// extractStorefrontToken validates a GraphQL response and returns the storefront access token.
// Returns an error if the response contains GraphQL errors, user errors, or an empty token.
func extractStorefrontToken(gqlResp *storefrontGQLResponse) (string, error) {
	if len(gqlResp.Errors) > 0 {
		return "", fmt.Errorf("storefront token graphql error: %s", gqlResp.Errors[0].Message)
	}
	create := gqlResp.Data.StorefrontAccessTokenCreate
	if len(create.UserErrors) > 0 {
		return "", fmt.Errorf("storefront token error: %s", create.UserErrors[0].Message)
	}
	token := create.StorefrontAccessToken.AccessToken
	if token == "" {
		return "", fmt.Errorf("storefront token creation returned empty token")
	}
	return token, nil
}
