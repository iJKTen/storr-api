package product

import (
	"errors"
	"net/http"

	"storr-api/internal/api"
)

var (
	// ErrProductNotFound is returned when a product lookup finds no matching item.
	ErrProductNotFound = errors.New("product not found")

	// ErrInvalidProductData is returned when product data fails validation.
	ErrInvalidProductData = errors.New("invalid product data")

	// ErrImportFailed is returned when a bulk import cannot be processed.
	ErrImportFailed = errors.New("product import failed")

	// ErrSyncFailed is returned when a sync operation with Shopify fails.
	ErrSyncFailed = errors.New("product sync failed")
)

// WriteProductError maps domain errors to safe HTTP responses.
// Internal details are logged server-side and never exposed to the client.
func WriteProductError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrProductNotFound) {
		api.WriteError(w, http.StatusNotFound, "Product not found", "NOT_FOUND")
		return
	}
	if errors.Is(err, ErrInvalidProductData) {
		api.WriteError(w, http.StatusBadRequest, "Invalid product data", "VALIDATION_ERROR")
		return
	}
	if errors.Is(err, ErrSyncFailed) {
		api.WriteError(w, http.StatusBadGateway, "Shopify sync failed", "SYNC_FAILED")
		return
	}
	api.WriteInternalError(w, "product operation failed", err)
}
