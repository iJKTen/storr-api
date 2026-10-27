package store

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"storr-api/internal/api"
	"storr-api/internal/auth"

	"github.com/go-chi/chi/v5"
)

var shopifyRegex = regexp.MustCompile(`^https://[a-zA-Z0-9-]+\.myshopify\.com$`)

func normalizeShopifyURL(raw string) string {
	u := strings.TrimSpace(raw)
	u = strings.TrimRight(u, "/")
	return u
}

// CleanupFunc is called after a store is deleted to clean up related resources.
type CleanupFunc func(ctx context.Context, userID, storeID string) error

// Handler provides HTTP endpoints for CRUD operations on stores.
type Handler struct {
	service *Service
	cleanup CleanupFunc
}

// NewHandler creates a store Handler backed by the given store service.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// SetCleanup registers a function to clean up related resources after store deletion.
func (h *Handler) SetCleanup(fn CleanupFunc) {
	h.cleanup = fn
}

// List returns all stores belonging to the authenticated user.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromRequest(r)
	stores, err := h.service.List(r.Context(), user.ID)
	if err != nil {
		api.WriteInternalError(w, "Failed to list stores", err)
		return
	}
	data, err := json.Marshal(stores)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

// Create validates input and creates a new store for the authenticated user.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromRequest(r)
	req, ok := decodeAndValidateCreate(w, r)
	if !ok {
		return
	}
	store, err := h.service.Create(r.Context(), user.ID, *req)
	if err != nil {
		api.WriteInternalError(w, "Failed to create store", err)
		return
	}
	data, err := json.Marshal(store)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusCreated, data)
}

// Get returns a single store by ID for the authenticated user.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromRequest(r)
	storeID := chi.URLParam(r, "id")
	store, err := h.service.Get(r.Context(), user.ID, storeID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	data, err := json.Marshal(store)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

// Update validates input and applies changes to an existing store.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromRequest(r)
	storeID := chi.URLParam(r, "id")
	req, ok := decodeAndValidateUpdate(w, r)
	if !ok {
		return
	}
	store, err := h.service.Update(r.Context(), user.ID, storeID, *req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	data, err := json.Marshal(store)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

// Delete removes a store and triggers cleanup of related resources.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromRequest(r)
	storeID := chi.URLParam(r, "id")
	if err := h.service.Delete(r.Context(), user.ID, storeID); err != nil {
		writeStoreError(w, err)
		return
	}
	if h.cleanup != nil {
		h.cleanup(r.Context(), user.ID, storeID)
	}
	api.WriteSuccessOK(w)
}

func decodeAndValidateCreate(w http.ResponseWriter, r *http.Request) (*CreateStoreRequest, bool) {
	req, err := decodeCreateStoreRequest(r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body", "VALIDATION_ERROR")
		return nil, false
	}
	if details := validateCreateRequest(*req); len(details) > 0 {
		api.WriteValidationError(w, details)
		return nil, false
	}
	return req, true
}

func decodeAndValidateUpdate(w http.ResponseWriter, r *http.Request) (*UpdateStoreRequest, bool) {
	req, err := decodeUpdateStoreRequest(r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body", "VALIDATION_ERROR")
		return nil, false
	}
	if details := validateUpdateRequest(*req); len(details) > 0 {
		api.WriteValidationError(w, details)
		return nil, false
	}
	return req, true
}

func decodeCreateStoreRequest(r *http.Request) (*CreateStoreRequest, error) {
	var req CreateStoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	req.Name = strings.TrimSpace(req.Name)
	req.ShopifyURL = normalizeShopifyURL(req.ShopifyURL)
	return &req, nil
}

func decodeUpdateStoreRequest(r *http.Request) (*UpdateStoreRequest, error) {
	var req UpdateStoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	req.Name = strings.TrimSpace(req.Name)
	req.ShopifyURL = normalizeShopifyURL(req.ShopifyURL)
	return &req, nil
}

func validateCreateRequest(req CreateStoreRequest) map[string]string {
	details := make(map[string]string)
	if req.Name == "" {
		details["name"] = "Store name is required"
	}
	if req.ShopifyURL != "" && !shopifyRegex.MatchString(req.ShopifyURL) {
		details["shopifyUrl"] = "Must be a valid .myshopify.com URL"
	}
	if req.ProjectType == "theme" && req.ShopifyURL == "" {
		details["shopifyUrl"] = "Shopify URL is required for theme stores"
	}
	return details
}

func validateUpdateRequest(req UpdateStoreRequest) map[string]string {
	details := make(map[string]string)
	if req.Name == "" {
		details["name"] = "Store name is required"
	}
	if req.ShopifyURL != "" && !shopifyRegex.MatchString(req.ShopifyURL) {
		details["shopifyUrl"] = "Must be a valid .myshopify.com URL"
	}
	return details
}

func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrStoreNotFound) {
		api.WriteError(w, http.StatusNotFound, "Store not found", "NOT_FOUND")
		return
	}
	api.WriteInternalError(w, "Failed to process store request", err)
}
