package integration

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"storr-api/internal/store"
)

// CallbackHandler handles OAuth callback redirects from Figma and Shopify.
type CallbackHandler struct {
	stateStore   *OAuthStateStore
	figmaOAuth   *FigmaOAuthService
	shopifyOAuth *ShopifyOAuthService
	frontendURL  string
}

// NewCallbackHandler creates a CallbackHandler for processing OAuth redirects.
func NewCallbackHandler(stateStore *OAuthStateStore, figmaOAuth *FigmaOAuthService, shopifyOAuth *ShopifyOAuthService, frontendURL string) *CallbackHandler {
	return &CallbackHandler{
		stateStore:   stateStore,
		figmaOAuth:   figmaOAuth,
		shopifyOAuth: shopifyOAuth,
		frontendURL:  frontendURL,
	}
}

// figmaCallbackParams holds validated parameters from a Figma OAuth callback.
type figmaCallbackParams struct {
	Code       string
	OAuthState *OAuthState
}

// validateFigmaCallback extracts and validates all parameters from the Figma
// OAuth callback request, including state lookup and deletion.
func (h *CallbackHandler) validateFigmaCallback(r *http.Request) (*figmaCallbackParams, error) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		return nil, fmt.Errorf("missing code or state parameter")
	}
	oauthState, err := h.stateStore.Get(r.Context(), state)
	if err != nil {
		return nil, fmt.Errorf("failed to get oauth state: %w", err)
	}
	if oauthState == nil || oauthState.Provider != store.ProviderFigma {
		return nil, fmt.Errorf("invalid or expired state")
	}
	if err := h.stateStore.Delete(r.Context(), state); err != nil {
		slog.Warn("failed to delete oauth state after use", "error", err)
	}
	return &figmaCallbackParams{Code: code, OAuthState: oauthState}, nil
}

// FigmaCallback completes the Figma OAuth flow by exchanging the code for tokens.
func (h *CallbackHandler) FigmaCallback(w http.ResponseWriter, r *http.Request) {
	params, err := h.validateFigmaCallback(r)
	if err != nil {
		slog.Error("figma callback validation", "error", err)
		h.redirectError(w, r, "invalid_request")
		return
	}
	if err := h.figmaOAuth.CompleteWithCode(r.Context(), params.OAuthState.CustomerID, params.OAuthState.StoreID, params.Code); err != nil {
		slog.Error("figma oauth complete failed", "error", err)
		h.redirectError(w, r, "failed to complete oauth")
		return
	}
	h.redirectSuccess(w, r, store.ProviderFigma)
}

// shopifyCallbackParams holds validated parameters from a Shopify OAuth callback.
type shopifyCallbackParams struct {
	Code       string
	Shop       string
	OAuthState *OAuthState
}

// validateShopifyCallback extracts and validates all parameters from the Shopify
// OAuth callback request, including state lookup, HMAC verification, and shop domain validation.
func (h *CallbackHandler) validateShopifyCallback(r *http.Request) (*shopifyCallbackParams, error) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	shopRaw := r.URL.Query().Get("shop")
	if code == "" || state == "" || shopRaw == "" {
		return nil, fmt.Errorf("missing required parameters")
	}
	if err := ValidateShopDomain(shopRaw); err != nil {
		return nil, err
	}
	shop := "https://" + shopRaw
	oauthState, err := h.stateStore.Get(r.Context(), state)
	if err != nil {
		return nil, fmt.Errorf("failed to get oauth state: %w", err)
	}
	if oauthState == nil || oauthState.Provider != store.ProviderShopify {
		return nil, fmt.Errorf("invalid or expired state")
	}
	if oauthState.Shop != shop {
		return nil, fmt.Errorf("shop mismatch: callback shop does not match OAuth state")
	}
	if !h.shopifyOAuth.ValidateHMAC(r.URL.Query()) {
		return nil, fmt.Errorf("invalid HMAC signature")
	}
	if err := h.stateStore.Delete(r.Context(), state); err != nil {
		slog.Warn("failed to delete oauth state after use", "error", err)
	}
	return &shopifyCallbackParams{Code: code, Shop: shop, OAuthState: oauthState}, nil
}

// ShopifyCallback completes the Shopify OAuth flow after HMAC validation.
func (h *CallbackHandler) ShopifyCallback(w http.ResponseWriter, r *http.Request) {
	params, err := h.validateShopifyCallback(r)
	if err != nil {
		slog.Error("shopify callback validation", "error", err)
		h.redirectError(w, r, "invalid_request")
		return
	}
	if err := h.shopifyOAuth.CompleteWithCode(r.Context(), ShopifyOAuthCompletion{
		CustomerID: params.OAuthState.CustomerID,
		StoreID:    params.OAuthState.StoreID,
		Shop:       params.Shop,
		Code:       params.Code,
	}); err != nil {
		slog.Error("shopify oauth complete failed", "error", err)
		h.redirectError(w, r, "failed to complete oauth")
		return
	}
	h.redirectSuccess(w, r, store.ProviderShopify)
}

func (h *CallbackHandler) redirectSuccess(w http.ResponseWriter, r *http.Request, provider string) {
	target := fmt.Sprintf("%s/oauth/callback?provider=%s&status=success", h.frontendURL, url.QueryEscape(provider))
	http.Redirect(w, r, target, http.StatusFound)
}

func (h *CallbackHandler) redirectError(w http.ResponseWriter, r *http.Request, errMsg string) {
	target := fmt.Sprintf("%s/oauth/callback?error=%s", h.frontendURL, url.QueryEscape(errMsg))
	http.Redirect(w, r, target, http.StatusFound)
}
