package product

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Service orchestrates product operations, applying validation and defaults
// before delegating persistence to the repository.
type Service struct {
	repo                   *Repository
	batchUpdateMaxProducts int
}

// NewService creates a product service backed by the given repository.
func NewService(repo *Repository, batchUpdateMaxProducts int) *Service {
	return &Service{repo: repo, batchUpdateMaxProducts: batchUpdateMaxProducts}
}

// Create validates the request, assigns IDs and defaults, and persists a new product.
func (s *Service) Create(ctx context.Context, storeID string, req CreateRequest) (*Product, error) {
	if err := validateCreateRequest(req); err != nil {
		return nil, err
	}
	p := buildProduct(storeID, req)
	if p.Status == StatusActive {
		p.Published = true
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("create product: %w", err)
	}
	return &p, nil
}

// Get retrieves a single product by store and product ID.
func (s *Service) Get(ctx context.Context, storeID, productID string) (*Product, error) {
	return s.repo.Get(ctx, storeID, productID)
}

// List returns a page of products for a store in the given direction.
func (s *Service) List(ctx context.Context, storeID, cursor string, limit int, direction PageDirection) (*ListResult, error) {
	return s.repo.List(ctx, storeID, cursor, limit, direction)
}

// Update validates the request, applies changes, and persists the product.
func (s *Service) Update(ctx context.Context, storeID, productID string, req UpdateRequest) (*Product, error) {
	existing, err := s.repo.Get(ctx, storeID, productID)
	if err != nil {
		return nil, err
	}
	if err := validateUpdateRequest(req); err != nil {
		return nil, err
	}
	updated := applyUpdate(*existing, req)
	if err := s.repo.Update(ctx, updated); err != nil {
		return nil, fmt.Errorf("update product: %w", err)
	}
	return &updated, nil
}

// Delete removes a product by store and product ID.
func (s *Service) Delete(ctx context.Context, storeID, productID string) error {
	if _, err := s.repo.Get(ctx, storeID, productID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, storeID, productID)
}

// BatchUpdate applies partial updates to multiple products. Products that fail
// validation or lookup are collected in the Failed list; successful products are
// persisted via BatchWrite. Returns a result with both Updated and Failed lists.
func (s *Service) BatchUpdate(ctx context.Context, storeID string, items []BatchUpdateItem) (*BatchUpdateResult, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("%w: batch update requires at least one item", ErrInvalidProductData)
	}
	if s.batchUpdateMaxProducts > 0 && len(items) > s.batchUpdateMaxProducts {
		return nil, fmt.Errorf("%w: batch update exceeds maximum of %d products", ErrInvalidProductData, s.batchUpdateMaxProducts)
	}

	// Validate productIDs: non-empty and no duplicates.
	seen := make(map[string]bool, len(items))
	productIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item.ProductID == "" {
			return nil, fmt.Errorf("%w: productId is required for each item", ErrInvalidProductData)
		}
		if seen[item.ProductID] {
			return nil, fmt.Errorf("%w: duplicate productId %s", ErrInvalidProductData, item.ProductID)
		}
		seen[item.ProductID] = true
		productIDs = append(productIDs, item.ProductID)
	}

	existing, err := s.repo.BatchGet(ctx, storeID, productIDs)
	if err != nil {
		return nil, fmt.Errorf("batch get products: %w", err)
	}

	var updated []Product
	result := &BatchUpdateResult{
		Updated: []string{},
		Failed:  []BatchUpdateError{},
	}

	for _, item := range items {
		product, ok := existing[item.ProductID]
		if !ok {
			result.Failed = append(result.Failed, BatchUpdateError{
				ProductID: item.ProductID,
				Message:   "product not found",
			})
			continue
		}

		merged, err := applyPartialUpdate(product, item)
		if err != nil {
			slog.ErrorContext(ctx, "batch update partial apply failed",
				"productId", item.ProductID, "storeId", storeID, "error", err)
			result.Failed = append(result.Failed, BatchUpdateError{
				ProductID: item.ProductID,
				Message:   sanitizeBatchErr(err),
			})
			continue
		}

		if err := validateMergedProduct(merged); err != nil {
			result.Failed = append(result.Failed, BatchUpdateError{
				ProductID: item.ProductID,
				Message:   sanitizeBatchErr(err),
			})
			continue
		}

		updated = append(updated, merged)
		result.Updated = append(result.Updated, item.ProductID)
	}

	if len(updated) > 0 {
		if err := s.repo.BatchWrite(ctx, updated); err != nil {
			return nil, fmt.Errorf("batch write products: %w", err)
		}
	}

	return result, nil
}

func applyPartialUpdate(existing Product, item BatchUpdateItem) (Product, error) {
	if item.Handle != nil {
		existing.Handle = *item.Handle
	}
	if item.Title != nil {
		existing.Title = *item.Title
	}
	if item.BodyHTML != nil {
		existing.BodyHTML = *item.BodyHTML
	}
	if item.Vendor != nil {
		existing.Vendor = *item.Vendor
	}
	if item.ProductType != nil {
		existing.ProductType = *item.ProductType
	}
	if item.Tags != nil {
		existing.Tags = *item.Tags
	}
	if item.Status != nil {
		existing.Status = resolveStatus(*item.Status)
	}
	if item.Published != nil {
		existing.Published = *item.Published
	}
	if item.PublishedScope != nil {
		existing.PublishedScope = *item.PublishedScope
	}
	if item.TemplateSuffix != nil {
		existing.TemplateSuffix = *item.TemplateSuffix
	}
	if item.SEOTitle != nil {
		existing.SEOTitle = *item.SEOTitle
	}
	if item.SEODescription != nil {
		existing.SEODescription = *item.SEODescription
	}
	if item.GiftCard != nil {
		existing.GiftCard = *item.GiftCard
	}
	if item.Metafields != nil {
		existing.Metafields = *item.Metafields
	}
	if item.Options != nil {
		existing.Options = *item.Options
	}
	if item.Variants != nil {
		merged, err := mergePartialVariants(existing.Variants, *item.Variants)
		if err != nil {
			return Product{}, err
		}
		existing.Variants = merged
	}
	existing.UpdatedAt = time.Now().UTC()
	if existing.SyncStatus == SyncSynced {
		existing.SyncStatus = SyncPending
	}
	return existing, nil
}

func mergePartialVariants(existing []Variant, patches []BatchUpdateVariant) ([]Variant, error) {
	// Check for duplicate variantIds in patches.
	seen := make(map[string]bool, len(patches))
	for _, p := range patches {
		if p.VariantID == "" {
			return nil, fmt.Errorf("%w: variantId is required for each variant patch", ErrInvalidProductData)
		}
		if seen[p.VariantID] {
			return nil, fmt.Errorf("%w: duplicate variantId %s", ErrInvalidProductData, p.VariantID)
		}
		seen[p.VariantID] = true
	}

	// Build lookup map of existing variants.
	variantMap := make(map[string]int, len(existing))
	for i, v := range existing {
		variantMap[v.VariantID] = i
	}

	// Apply patches to existing variants.
	result := make([]Variant, len(existing))
	copy(result, existing)
	for _, patch := range patches {
		idx, ok := variantMap[patch.VariantID]
		if !ok {
			return nil, fmt.Errorf("%w: variant %s not found", ErrInvalidProductData, patch.VariantID)
		}
		result[idx] = applyPartialVariant(result[idx], patch)
	}
	return result, nil
}

func applyPartialVariant(existing Variant, patch BatchUpdateVariant) Variant {
	if patch.Title != nil {
		existing.Title = *patch.Title
	}
	if patch.SKU != nil {
		existing.SKU = *patch.SKU
	}
	if patch.Barcode != nil {
		existing.Barcode = *patch.Barcode
	}
	if patch.Price != nil {
		existing.Price = *patch.Price
	}
	if patch.CompareAtPrice != nil {
		existing.CompareAtPrice = *patch.CompareAtPrice
	}
	if patch.CostPerItem != nil {
		existing.CostPerItem = *patch.CostPerItem
	}
	if patch.Position != nil {
		existing.Position = *patch.Position
	}
	if patch.InventoryQty != nil {
		existing.InventoryQty = *patch.InventoryQty
	}
	if patch.InventoryPolicy != nil {
		existing.InventoryPolicy = *patch.InventoryPolicy
	}
	if patch.InventoryManagement != nil {
		existing.InventoryManagement = *patch.InventoryManagement
	}
	if patch.FulfillmentService != nil {
		existing.FulfillmentService = *patch.FulfillmentService
	}
	if patch.Option1 != nil {
		existing.Option1 = *patch.Option1
	}
	if patch.Option2 != nil {
		existing.Option2 = *patch.Option2
	}
	if patch.Option3 != nil {
		existing.Option3 = *patch.Option3
	}
	if patch.Taxable != nil {
		existing.Taxable = *patch.Taxable
	}
	if patch.TaxCode != nil {
		existing.TaxCode = *patch.TaxCode
	}
	if patch.RequiresShipping != nil {
		existing.RequiresShipping = *patch.RequiresShipping
	}
	if patch.Weight != nil {
		existing.Weight = *patch.Weight
	}
	if patch.WeightUnit != nil {
		existing.WeightUnit = *patch.WeightUnit
	}
	if patch.Grams != nil {
		existing.Grams = *patch.Grams
	}
	if patch.ImageID != nil {
		existing.ImageID = *patch.ImageID
	}
	return existing
}

func validateMergedProduct(p Product) error {
	if p.Title == "" {
		return fmt.Errorf("%w: title is required", ErrInvalidProductData)
	}
	if len(p.Variants) > MaxVariantsPerProduct {
		return fmt.Errorf("%w: too many variants (max %d)", ErrInvalidProductData, MaxVariantsPerProduct)
	}
	if len(p.Options) > MaxOptionsPerProduct {
		return fmt.Errorf("%w: too many options (max %d)", ErrInvalidProductData, MaxOptionsPerProduct)
	}
	for _, v := range p.Variants {
		if v.Price == "" {
			return fmt.Errorf("%w: variant %s price is required", ErrInvalidProductData, v.VariantID)
		}
	}
	return nil
}

// sanitizeBatchErr returns a safe client-facing message for batch update errors.
// Validation errors are our own text and safe to expose.
// Infrastructure errors are hidden behind a generic message.
func sanitizeBatchErr(err error) string {
	if errors.Is(err, ErrInvalidProductData) {
		return err.Error()
	}
	if errors.Is(err, ErrProductNotFound) {
		return "product not found"
	}
	return batchUpdateErrGeneric
}

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func buildProduct(storeID string, req CreateRequest) Product {
	now := time.Now().UTC()
	p := Product{
		StoreID:        storeID,
		ProductID:      ProductIDPrefix + uuid.NewString(),
		Handle:         req.Handle,
		Title:          req.Title,
		BodyHTML:        req.BodyHTML,
		Vendor:         req.Vendor,
		ProductType:    req.ProductType,
		Tags:           req.Tags,
		Status:         resolveStatus(req.Status),
		Published:      derefBool(req.Published),
		PublishedScope: req.PublishedScope,
		TemplateSuffix: req.TemplateSuffix,
		SEOTitle:       req.SEOTitle,
		SEODescription: req.SEODescription,
		GiftCard:       derefBool(req.GiftCard),
		Metafields:     req.Metafields,
		Options:        req.Options,
		SyncStatus:     SyncLocal,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	p.Variants = assignVariantIDs(req.Variants)
	return p
}

func applyUpdate(existing Product, req UpdateRequest) Product {
	existing.Handle = req.Handle
	existing.Title = req.Title
	existing.BodyHTML = req.BodyHTML
	existing.Vendor = req.Vendor
	existing.ProductType = req.ProductType
	existing.Tags = req.Tags
	existing.Status = resolveStatus(req.Status)
	if req.Published != nil {
		existing.Published = *req.Published
	}
	existing.PublishedScope = req.PublishedScope
	existing.TemplateSuffix = req.TemplateSuffix
	existing.SEOTitle = req.SEOTitle
	existing.SEODescription = req.SEODescription
	if req.GiftCard != nil {
		existing.GiftCard = *req.GiftCard
	}
	existing.Metafields = req.Metafields
	existing.Options = req.Options
	existing.Variants = mergeVariants(existing.Variants, req.Variants)
	existing.UpdatedAt = time.Now().UTC()
	if existing.SyncStatus == SyncSynced {
		existing.SyncStatus = SyncPending
	}
	return existing
}

func assignVariantIDs(variants []Variant) []Variant {
	result := make([]Variant, len(variants))
	for i, v := range variants {
		if v.VariantID == "" {
			v.VariantID = VariantIDPrefix + uuid.NewString()
		}
		v = applyVariantDefaults(v)
		result[i] = v
	}
	return result
}

func mergeVariants(existing, incoming []Variant) []Variant {
	result := make([]Variant, len(incoming))
	for i, v := range incoming {
		if v.VariantID == "" {
			v.VariantID = VariantIDPrefix + uuid.NewString()
		}
		v = applyVariantDefaults(v)
		result[i] = v
	}
	return result
}

func applyVariantDefaults(v Variant) Variant {
	if v.InventoryPolicy == "" {
		v.InventoryPolicy = DefaultInventoryPolicy
	}
	if v.FulfillmentService == "" {
		v.FulfillmentService = DefaultFulfillmentService
	}
	if v.WeightUnit == "" {
		v.WeightUnit = DefaultWeightUnit
	}
	return v
}

func resolveStatus(raw string) ProductStatus {
	switch ProductStatus(raw) {
	case StatusActive:
		return StatusActive
	case StatusArchived:
		return StatusArchived
	default:
		return StatusDraft
	}
}

func validateCreateRequest(req CreateRequest) error {
	if req.Title == "" {
		return fmt.Errorf("%w: title is required", ErrInvalidProductData)
	}
	if len(req.Variants) > MaxVariantsPerProduct {
		return fmt.Errorf("%w: too many variants (max %d)", ErrInvalidProductData, MaxVariantsPerProduct)
	}
	if len(req.Options) > MaxOptionsPerProduct {
		return fmt.Errorf("%w: too many options (max %d)", ErrInvalidProductData, MaxOptionsPerProduct)
	}
	return validateVariantPrices(req.Variants)
}

func validateUpdateRequest(req UpdateRequest) error {
	if req.Title == "" {
		return fmt.Errorf("%w: title is required", ErrInvalidProductData)
	}
	if len(req.Variants) > MaxVariantsPerProduct {
		return fmt.Errorf("%w: too many variants (max %d)", ErrInvalidProductData, MaxVariantsPerProduct)
	}
	if len(req.Options) > MaxOptionsPerProduct {
		return fmt.Errorf("%w: too many options (max %d)", ErrInvalidProductData, MaxOptionsPerProduct)
	}
	return validateVariantPrices(req.Variants)
}

func validateVariantPrices(variants []Variant) error {
	for i, v := range variants {
		if v.Price == "" {
			return fmt.Errorf("%w: variant %d price is required", ErrInvalidProductData, i+1)
		}
	}
	return nil
}
