package image

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"storr-api/internal/api"
	"storr-api/internal/auth"
	"storr-api/internal/product"

	"github.com/go-chi/chi/v5"
)

// ShopifyImageSyncer defines the Shopify image operations used by the image handler.
type ShopifyImageSyncer interface {
	DeleteShopifyImage(ctx context.Context, op product.ShopifyImageOp, img product.Image) error
	ReorderShopifyImages(ctx context.Context, op product.ShopifyImageOp, entries []product.ReorderImageEntry) error
}

// Handler provides HTTP endpoints for product image operations.
type Handler struct {
	imageService *Service
	syncService  ShopifyImageSyncer
}

// NewHandler creates an image handler backed by the given image and sync services.
func NewHandler(imageService *Service, syncService ShopifyImageSyncer) *Handler {
	return &Handler{imageService: imageService, syncService: syncService}
}

// GenerateUploadURL creates a presigned S3 PUT URL for direct client-side upload.
func (h *Handler) GenerateUploadURL(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "id")
	productID := chi.URLParam(r, "productId")
	req, err := decodeUploadURLRequest(r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body", "INVALID_BODY")
		return
	}
	resp, err := h.imageService.GenerateUploadURL(r.Context(), storeID, productID, *req)
	if err != nil {
		product.WriteProductError(w, err)
		return
	}
	writeUploadURLJSON(w, resp)
}

// ConfirmUpload confirms a client-side S3 upload and adds the image to the product.
func (h *Handler) ConfirmUpload(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "id")
	productID := chi.URLParam(r, "productId")
	req, err := decodeConfirmImageRequest(r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body", "INVALID_BODY")
		return
	}
	p, err := h.imageService.ConfirmUpload(r.Context(), storeID, productID, *req)
	if err != nil {
		product.WriteProductError(w, err)
		return
	}
	writeProductJSON(w, http.StatusOK, p)
}

// UpdateImage modifies image properties (name, altText, position).
func (h *Handler) UpdateImage(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "id")
	productID := chi.URLParam(r, "productId")
	imageID := chi.URLParam(r, "imageId")
	req, err := decodeUpdateImageRequest(r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body", "INVALID_BODY")
		return
	}
	ref := ImageRef{StoreID: storeID, ProductID: productID, ImageID: imageID}
	p, err := h.imageService.UpdateImage(r.Context(), ref, *req)
	if err != nil {
		product.WriteProductError(w, err)
		return
	}
	writeProductJSON(w, http.StatusOK, p)
}

// DeleteImage removes an image from the product.
func (h *Handler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "id")
	productID := chi.URLParam(r, "productId")
	imageID := chi.URLParam(r, "imageId")
	p, deletedImage, err := h.imageService.DeleteImage(r.Context(), storeID, productID, imageID)
	if err != nil {
		product.WriteProductError(w, err)
		return
	}
	h.deleteFromShopify(r, storeID, p, deletedImage)
	writeProductJSON(w, http.StatusOK, p)
}

// ReorderImages updates the position of each specified image.
func (h *Handler) ReorderImages(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "id")
	productID := chi.URLParam(r, "productId")
	entries, err := decodeReorderRequest(r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body", "INVALID_BODY")
		return
	}
	p, err := h.imageService.ReorderImages(r.Context(), storeID, productID, entries)
	if err != nil {
		product.WriteProductError(w, err)
		return
	}
	h.reorderOnShopify(r, storeID, p, entries)
	writeProductJSON(w, http.StatusOK, p)
}

func (h *Handler) deleteFromShopify(r *http.Request, storeID string, p *product.Product, deletedImage *product.Image) {
	if deletedImage == nil || !product.IsSyncedImage(*deletedImage) {
		return
	}
	user := auth.UserFromRequest(r)
	if user == nil {
		return
	}
	op := product.ShopifyImageOp{UserID: user.ID, StoreID: storeID, Product: p}
	if err := h.syncService.DeleteShopifyImage(r.Context(), op, *deletedImage); err != nil {
		slog.Warn("failed to delete image from Shopify", "error", err)
	}
}

func (h *Handler) reorderOnShopify(r *http.Request, storeID string, p *product.Product, entries []product.ReorderImageEntry) {
	if p.ShopifyID == "" {
		return
	}
	user := auth.UserFromRequest(r)
	if user == nil {
		return
	}
	op := product.ShopifyImageOp{UserID: user.ID, StoreID: storeID, Product: p}
	if err := h.syncService.ReorderShopifyImages(r.Context(), op, entries); err != nil {
		slog.Warn("failed to reorder images on Shopify", "error", err)
	}
}

func decodeUploadURLRequest(r *http.Request) (*product.UploadURLRequest, error) {
	var req product.UploadURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	return &req, nil
}

func decodeConfirmImageRequest(r *http.Request) (*product.ConfirmImageRequest, error) {
	var req product.ConfirmImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	return &req, nil
}

func decodeUpdateImageRequest(r *http.Request) (*product.UpdateImageRequest, error) {
	var req product.UpdateImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	return &req, nil
}

func decodeReorderRequest(r *http.Request) ([]product.ReorderImageEntry, error) {
	var entries []product.ReorderImageEntry
	if err := json.NewDecoder(r.Body).Decode(&entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func writeUploadURLJSON(w http.ResponseWriter, resp *product.UploadURLResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

func writeProductJSON(w http.ResponseWriter, status int, p *product.Product) {
	data, err := json.Marshal(p)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, status, data)
}
