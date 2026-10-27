package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"storr-api/internal/api"
	"storr-api/internal/auth"
	"storr-api/internal/store"

	"github.com/go-chi/chi/v5"
)

// Handler provides HTTP endpoints for managing store integrations.
type Handler struct {
	figmaOAuth   *FigmaOAuthService
	shopifyOAuth *ShopifyOAuthService
	stores       *store.Service
}

// NewHandler creates an integration Handler with the given OAuth services and store service.
func NewHandler(figmaOAuth *FigmaOAuthService, shopifyOAuth *ShopifyOAuthService, stores *store.Service) *Handler {
	return &Handler{
		figmaOAuth:   figmaOAuth,
		shopifyOAuth: shopifyOAuth,
		stores:       stores,
	}
}

// FigmaStart initiates the Figma OAuth flow and returns the authorization URL.
func (h *Handler) FigmaStart(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromRequest(r)
	storeID := chi.URLParam(r, "id")
	resp, err := h.figmaOAuth.Start(r.Context(), user.ID, storeID)
	if err != nil {
		api.WriteInternalError(w, "Failed to start Figma OAuth", err)
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

// decodeShopifyStartRequest decodes and validates the shop field from the request body.
// Returns the shop as a full https:// URL (e.g. "https://my-store.myshopify.com").
func decodeShopifyStartRequest(r *http.Request) (string, error) {
	var req StartShopifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return "", err
	}
	shop := strings.TrimSpace(req.Shop)
	if shop == "" {
		return "", fmt.Errorf("shop is required")
	}
	// Normalize to full URL so downstream code and state comparisons are consistent.
	shop = strings.TrimPrefix(shop, "https://")
	shop = strings.TrimPrefix(shop, "http://")
	return "https://" + shop, nil
}

// ShopifyStart initiates the Shopify OAuth flow and returns the install URL.
func (h *Handler) ShopifyStart(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromRequest(r)
	storeID := chi.URLParam(r, "id")
	shop, err := decodeShopifyStartRequest(r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body", "VALIDATION_ERROR")
		return
	}
	resp, err := h.shopifyOAuth.Start(r.Context(), user.ID, storeID, shop)
	if err != nil {
		api.WriteInternalError(w, "Failed to start Shopify OAuth", err)
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

// ListStoreIntegrations returns all integration configs for a store.
func (h *Handler) ListStoreIntegrations(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromRequest(r)
	storeID := chi.URLParam(r, "id")
	integrations, err := h.stores.GetIntegrations(r.Context(), user.ID, storeID)
	if err != nil {
		api.WriteError(w, http.StatusNotFound, "Store not found", "NOT_FOUND")
		return
	}
	data, err := json.Marshal(integrations)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

// UpdateStoreIntegration upserts a single integration config on a store.
func (h *Handler) UpdateStoreIntegration(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromRequest(r)
	storeID := chi.URLParam(r, "id")
	key := chi.URLParam(r, "key")
	var config store.IntegrationConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body", "VALIDATION_ERROR")
		return
	}
	if err := h.stores.PutIntegration(r.Context(), user.ID, storeID, store.IntegrationUpdate{Key: key, Config: config}); err != nil {
		api.WriteError(w, http.StatusNotFound, "Store not found", "NOT_FOUND")
		return
	}
	api.WriteStatusOK(w)
}

// DisableStoreIntegration removes an integration from a store.
func (h *Handler) DisableStoreIntegration(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromRequest(r)
	storeID := chi.URLParam(r, "id")
	key := chi.URLParam(r, "key")
	if err := h.stores.DeleteIntegration(r.Context(), user.ID, storeID, key); err != nil {
		api.WriteError(w, http.StatusNotFound, "Store not found", "NOT_FOUND")
		return
	}
	api.WriteStatusOK(w)
}
