package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"storr-api/internal/api"
	"storr-api/internal/auth"

	"github.com/go-chi/chi/v5"
)

// Handler provides HTTP endpoints for product CRUD and sync operations.
type Handler struct {
	service      *Service
	syncService  ProductSyncer
	generateXLSX func() (*bytes.Buffer, error)
	generateCSV  func() ([]byte, error)
	parseCSV     func(io.Reader, map[string]string) ([]Product, []ValidationError)
	parseXLSX    func(io.Reader, map[string]string) ([]Product, []ValidationError)
}

// HandlerOpts holds optional function dependencies for the product handler.
type HandlerOpts struct {
	GenerateXLSX func() (*bytes.Buffer, error)
	GenerateCSV  func() ([]byte, error)
	ParseCSV     func(io.Reader, map[string]string) ([]Product, []ValidationError)
	ParseXLSX    func(io.Reader, map[string]string) ([]Product, []ValidationError)
}

// NewHandler creates a product handler backed by the given service and sync service.
func NewHandler(service *Service, syncService ProductSyncer, opts HandlerOpts) *Handler {
	return &Handler{
		service:      service,
		syncService:  syncService,
		generateXLSX: opts.GenerateXLSX,
		generateCSV:  opts.GenerateCSV,
		parseCSV:     opts.ParseCSV,
		parseXLSX:    opts.ParseXLSX,
	}
}

// List returns a paginated list of products for the store.
// Supports bidirectional pagination via `after` (forward) and `before` (backward)
// query parameters. The legacy `cursor` parameter is treated as an alias for `after`.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "id")
	limit := parseLimit(r.URL.Query().Get("limit"))

	cursor, direction, err := resolveListCursor(r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_CURSOR")
		return
	}

	result, err := h.service.List(r.Context(), storeID, cursor, limit, direction)
	if err != nil {
		WriteProductError(w, err)
		return
	}
	writeListResponse(w, result)
}

// Create validates input and creates a new product for the store.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "id")
	req, err := decodeCreateRequest(r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body", "INVALID_BODY")
		return
	}
	product, err := h.service.Create(r.Context(), storeID, *req)
	if err != nil {
		WriteProductError(w, err)
		return
	}
	writeProductJSON(w, http.StatusCreated, product)
}

// Get returns a single product by ID.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "id")
	productID := chi.URLParam(r, "productId")
	product, err := h.service.Get(r.Context(), storeID, productID)
	if err != nil {
		WriteProductError(w, err)
		return
	}
	writeProductJSON(w, http.StatusOK, product)
}

// Update validates input and applies changes to an existing product.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "id")
	productID := chi.URLParam(r, "productId")
	req, err := decodeUpdateRequest(r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body", "INVALID_BODY")
		return
	}
	product, err := h.service.Update(r.Context(), storeID, productID, *req)
	if err != nil {
		WriteProductError(w, err)
		return
	}
	writeProductJSON(w, http.StatusOK, product)
}

// BatchUpdate applies partial updates to multiple products.
func (h *Handler) BatchUpdate(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "id")
	var items []BatchUpdateItem
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body", "INVALID_BODY")
		return
	}
	result, err := h.service.BatchUpdate(r.Context(), storeID, items)
	if err != nil {
		WriteProductError(w, err)
		return
	}
	data, err := json.Marshal(result)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

// Delete removes a product by ID.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	storeID := chi.URLParam(r, "id")
	productID := chi.URLParam(r, "productId")
	if err := h.service.Delete(r.Context(), storeID, productID); err != nil {
		WriteProductError(w, err)
		return
	}
	api.WriteSuccessOK(w)
}

// DownloadTemplate serves a downloadable product template in XLSX or CSV format.
// Query parameter: ?format=csv for CSV, defaults to XLSX.
func (h *Handler) DownloadTemplate(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("format") == "csv" {
		h.serveCSVTemplate(w)
		return
	}
	h.serveXLSXTemplate(w)
}

func (h *Handler) serveXLSXTemplate(w http.ResponseWriter) {
	buf, err := h.generateXLSX()
	if err != nil {
		api.WriteInternalError(w, "generate xlsx template", err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+TemplateXLSXFilename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

func (h *Handler) serveCSVTemplate(w http.ResponseWriter) {
	data, err := h.generateCSV()
	if err != nil {
		api.WriteInternalError(w, "generate csv template", err)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="`+TemplateCSVFilename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// Import handles bulk product import from a CSV or Excel file upload.
func (h *Handler) Import(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxImportFileBytes)
	storeID := chi.URLParam(r, "id")
	file, header, err := r.FormFile("file")
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "File upload is required", "MISSING_FILE")
		return
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			slog.Warn("failed to close import file", "error", cerr)
		}
	}()
	contentType := header.Header.Get("Content-Type")
	products, valErrors := h.parseImportFile(file, header.Filename, contentType)
	result := h.buildImportResult(r.Context(), storeID, products, valErrors)
	writeImportJSON(w, result)
}

func (h *Handler) parseImportFile(file io.Reader, filename string, contentType string) ([]Product, []ValidationError) {
	format := detectFileFormat(filename, contentType)
	switch format {
	case "csv":
		return h.parseCSV(file, nil)
	case "xlsx":
		return h.parseXLSX(file, nil)
	default:
		return nil, []ValidationError{{Row: 0, Message: "Unsupported file type. Use .csv or .xlsx"}}
	}
}

func detectFileFormat(filename string, contentType string) string {
	switch contentType {
	case "text/csv", "application/csv":
		return "csv"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return "xlsx"
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".csv":
		return "csv"
	case ".xlsx":
		return "xlsx"
	}
	return ""
}

func (h *Handler) buildImportResult(ctx context.Context, storeID string, products []Product, valErrors []ValidationError) ImportResult {
	imported, saveErrors := h.saveProducts(ctx, storeID, products)
	allErrors := make([]ValidationError, 0, len(valErrors)+len(saveErrors))
	allErrors = append(allErrors, valErrors...)
	allErrors = append(allErrors, saveErrors...)
	return ImportResult{
		Imported: imported,
		Skipped:  len(products) - imported,
		Errors:   allErrors,
	}
}

func (h *Handler) saveProducts(ctx context.Context, storeID string, products []Product) (int, []ValidationError) {
	var errs []ValidationError
	imported := 0
	for _, p := range products {
		if _, err := h.service.Create(ctx, storeID, productToCreateRequest(p)); err != nil {
			slog.ErrorContext(ctx, "import save failed",
				"handle", p.Handle, "storeId", storeID, "error", err)
			errs = append(errs, ValidationError{
				Column: p.Handle, Message: sanitizeImportErr(err),
			})
			continue
		}
		imported++
	}
	return imported, errs
}

// sanitizeImportErr returns a safe client-facing message.
// Validation errors are our own text and safe to expose.
// Infrastructure errors are hidden behind a generic message.
func sanitizeImportErr(err error) string {
	if errors.Is(err, ErrInvalidProductData) {
		return err.Error()
	}
	return importErrSaveFailed
}

func productToCreateRequest(p Product) CreateRequest {
	published := p.Published
	giftCard := p.GiftCard
	return CreateRequest{
		Handle:         p.Handle,
		Title:          p.Title,
		BodyHTML:       p.BodyHTML,
		Vendor:         p.Vendor,
		ProductType:    p.ProductType,
		Tags:           p.Tags,
		Status:         string(p.Status),
		Published:      &published,
		PublishedScope: p.PublishedScope,
		TemplateSuffix: p.TemplateSuffix,
		SEOTitle:       p.SEOTitle,
		SEODescription: p.SEODescription,
		GiftCard:       &giftCard,
		Metafields:     p.Metafields,
		Options:        p.Options,
		Variants:       p.Variants,
	}
}

func writeImportJSON(w http.ResponseWriter, result ImportResult) {
	data, err := json.Marshal(result)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

// SyncPull pulls all products from Shopify into the local store.
func (h *Handler) SyncPull(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "Missing authorization", "UNAUTHORIZED")
		return
	}
	storeID := chi.URLParam(r, "id")
	result, err := h.syncService.PullFromShopify(r.Context(), userID, storeID)
	if err != nil {
		WriteProductError(w, err)
		return
	}
	writeSyncJSON(w, result)
}

// SyncPush pushes all products to Shopify.
func (h *Handler) SyncPush(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "Missing authorization", "UNAUTHORIZED")
		return
	}
	storeID := chi.URLParam(r, "id")
	result, err := h.syncService.PushAllToShopify(r.Context(), userID, storeID)
	if err != nil {
		WriteProductError(w, err)
		return
	}
	writeSyncJSON(w, result)
}

// SyncPushOne pushes a single product to Shopify.
func (h *Handler) SyncPushOne(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "Missing authorization", "UNAUTHORIZED")
		return
	}
	storeID := chi.URLParam(r, "id")
	productID := chi.URLParam(r, "productId")
	if err := h.syncService.PushToShopify(r.Context(), userID, storeID, productID); err != nil {
		WriteProductError(w, err)
		return
	}
	api.WriteSuccessOK(w)
}

// GetShopifyVersion returns the current Shopify version of a product for conflict comparison.
func (h *Handler) GetShopifyVersion(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "Missing authorization", "UNAUTHORIZED")
		return
	}
	storeID := chi.URLParam(r, "id")
	productID := chi.URLParam(r, "productId")
	p, err := h.syncService.FetchShopifyVersion(r.Context(), userID, storeID, productID)
	if err != nil {
		WriteProductError(w, err)
		return
	}
	writeProductJSON(w, http.StatusOK, p)
}

// ResolveConflict resolves a sync conflict by keeping the local or Shopify version.
func (h *Handler) ResolveConflict(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserID(r)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "Missing authorization", "UNAUTHORIZED")
		return
	}
	storeID := chi.URLParam(r, "id")
	productID := chi.URLParam(r, "productId")
	req, err := decodeResolveRequest(r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "Invalid request body", "INVALID_BODY")
		return
	}
	ref := productRef{UserID: userID, StoreID: storeID, ProductID: productID}
	if err := h.resolveByStrategy(r.Context(), ref, req.Resolution); err != nil {
		WriteProductError(w, err)
		return
	}
	api.WriteSuccessOK(w)
}

// productRef identifies a product within a store for a specific user.
type productRef struct {
	UserID    string
	StoreID   string
	ProductID string
}

func (h *Handler) resolveByStrategy(ctx context.Context, ref productRef, resolution string) error {
	switch resolution {
	case ResolutionKeepLocal:
		return h.syncService.ResolveKeepLocal(ctx, ref.UserID, ref.StoreID, ref.ProductID)
	case ResolutionUseShopify:
		return h.syncService.ResolveUseShopify(ctx, ref.UserID, ref.StoreID, ref.ProductID)
	default:
		return fmt.Errorf("%w: invalid resolution", ErrInvalidProductData)
	}
}

func decodeResolveRequest(r *http.Request) (*ResolveConflictRequest, error) {
	var req ResolveConflictRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	return &req, nil
}

func writeSyncJSON(w http.ResponseWriter, result *SyncResult) {
	data, err := json.Marshal(result)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

func decodeCreateRequest(r *http.Request) (*CreateRequest, error) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Handle = strings.TrimSpace(req.Handle)
	return &req, nil
}

func decodeUpdateRequest(r *http.Request) (*UpdateRequest, error) {
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Handle = strings.TrimSpace(req.Handle)
	return &req, nil
}

func writeProductJSON(w http.ResponseWriter, status int, p *Product) {
	data, err := json.Marshal(p)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, status, data)
}

func writeListResponse(w http.ResponseWriter, result *ListResult) {
	resp := ListResponse{
		Products:   result.Products,
		NextCursor: result.NextCursor,
		PrevCursor: result.PrevCursor,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		api.WriteMarshalError(w, err)
		return
	}
	api.WriteJSON(w, http.StatusOK, data)
}

// resolveListCursor extracts the pagination cursor and direction from query parameters.
// The `after` and `before` parameters are mutually exclusive. The legacy `cursor`
// parameter is an alias for `after`.
func resolveListCursor(r *http.Request) (string, PageDirection, error) {
	after := r.URL.Query().Get("after")
	before := r.URL.Query().Get("before")
	legacy := r.URL.Query().Get("cursor")

	if after != "" && before != "" {
		return "", "", fmt.Errorf("after and before are mutually exclusive")
	}

	if before != "" {
		return before, PageBackward, nil
	}

	// Prefer explicit `after` over legacy `cursor`.
	if after != "" {
		return after, PageForward, nil
	}
	return legacy, PageForward, nil
}

func getUserID(r *http.Request) (string, bool) {
	user := auth.UserFromRequest(r)
	if user == nil || user.ID == "" {
		return "", false
	}
	return user.ID, true
}

func parseLimit(raw string) int {
	if raw == "" {
		return DefaultPageSize
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return DefaultPageSize
	}
	return n
}
