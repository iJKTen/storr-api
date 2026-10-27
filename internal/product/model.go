package product

import (
	"context"
	"time"
)

// ProductStatus represents the publication state of a product.
type ProductStatus string

const (
	StatusActive   ProductStatus = "active"
	StatusDraft    ProductStatus = "draft"
	StatusArchived ProductStatus = "archived"
)

// SyncStatus indicates the synchronization state between Storr and Shopify.
type SyncStatus string

const (
	SyncLocal    SyncStatus = "local"
	SyncSynced   SyncStatus = "synced"
	SyncPending  SyncStatus = "pending"
	SyncConflict SyncStatus = "conflict"
)

// Metafield represents a Shopify metafield key-value pair.
type Metafield struct {
	Namespace string `json:"namespace" dynamodbav:"namespace"`
	Key       string `json:"key"       dynamodbav:"key"`
	Value     string `json:"value"     dynamodbav:"value"`
	Type      string `json:"type"      dynamodbav:"type"`
}

// Product represents a product in the Storr catalog, optionally linked to Shopify.
type Product struct {
	StoreID        string        `json:"storeId"        dynamodbav:"storeId"`
	ProductID      string        `json:"productId"      dynamodbav:"productId"`
	Handle         string        `json:"handle"         dynamodbav:"handle"`
	Title          string        `json:"title"          dynamodbav:"title"`
	BodyHTML       string        `json:"bodyHtml"       dynamodbav:"bodyHtml"`
	Vendor         string        `json:"vendor"         dynamodbav:"vendor"`
	ProductType    string        `json:"productType"    dynamodbav:"productType"`
	Tags           []string      `json:"tags"           dynamodbav:"tags"`
	Status         ProductStatus `json:"status"         dynamodbav:"status"`
	Published      bool          `json:"published"      dynamodbav:"published"`
	PublishedScope string        `json:"publishedScope" dynamodbav:"publishedScope"`
	TemplateSuffix string        `json:"templateSuffix" dynamodbav:"templateSuffix"`
	SEOTitle       string        `json:"seoTitle,omitempty"       dynamodbav:"seoTitle,omitempty"`
	SEODescription string        `json:"seoDescription,omitempty" dynamodbav:"seoDescription,omitempty"`
	GiftCard       bool          `json:"giftCard"       dynamodbav:"giftCard"`
	Metafields     []Metafield   `json:"metafields,omitempty"     dynamodbav:"metafields,omitempty"`
	Options        []Option      `json:"options"        dynamodbav:"options"`
	Variants       []Variant     `json:"variants"       dynamodbav:"variants"`
	Images         []Image       `json:"images"         dynamodbav:"images"`
	ShopifyID      string        `json:"shopifyId"      dynamodbav:"shopifyId,omitempty"`
	SyncStatus     SyncStatus    `json:"syncStatus"     dynamodbav:"syncStatus"`
	PublishedAt    *time.Time    `json:"publishedAt,omitempty"  dynamodbav:"publishedAt,omitempty"`
	LastPushedAt   *time.Time    `json:"lastPushedAt,omitempty" dynamodbav:"lastPushedAt,omitempty"`
	LastPulledAt   *time.Time    `json:"lastPulledAt,omitempty" dynamodbav:"lastPulledAt,omitempty"`
	CreatedAt      time.Time     `json:"createdAt"      dynamodbav:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"      dynamodbav:"updatedAt"`
}

// Option defines a product option axis (e.g. "Color" or "Size") and its allowed values.
type Option struct {
	Name     string   `json:"name"     dynamodbav:"name"`
	Position int      `json:"position" dynamodbav:"position"`
	Values   []string `json:"values"   dynamodbav:"values"`
}

// Variant represents a product variant such as a specific size or color combination.
type Variant struct {
	VariantID           string  `json:"variantId"           dynamodbav:"variantId"`
	ShopifyVariantID    string  `json:"shopifyVariantId"    dynamodbav:"shopifyVariantId"`
	Title               string  `json:"title"               dynamodbav:"title"`
	SKU                 string  `json:"sku"                 dynamodbav:"sku"`
	Barcode             string  `json:"barcode"             dynamodbav:"barcode"`
	Price               string  `json:"price"               dynamodbav:"price"`
	CompareAtPrice      string  `json:"compareAtPrice"      dynamodbav:"compareAtPrice"`
	CostPerItem         string  `json:"costPerItem,omitempty" dynamodbav:"costPerItem,omitempty"`
	Position            int     `json:"position"            dynamodbav:"position"`
	InventoryQty        int     `json:"inventoryQty"        dynamodbav:"inventoryQty"`
	InventoryPolicy     string  `json:"inventoryPolicy"     dynamodbav:"inventoryPolicy"`
	InventoryManagement string  `json:"inventoryManagement" dynamodbav:"inventoryManagement"`
	FulfillmentService  string  `json:"fulfillmentService"  dynamodbav:"fulfillmentService"`
	Option1             string  `json:"option1"             dynamodbav:"option1"`
	Option2             string  `json:"option2"             dynamodbav:"option2"`
	Option3             string  `json:"option3"             dynamodbav:"option3"`
	Taxable             bool    `json:"taxable"             dynamodbav:"taxable"`
	TaxCode             string  `json:"taxCode"             dynamodbav:"taxCode"`
	RequiresShipping    bool    `json:"requiresShipping"    dynamodbav:"requiresShipping"`
	Weight              float64 `json:"weight"              dynamodbav:"weight"`
	WeightUnit          string  `json:"weightUnit"          dynamodbav:"weightUnit"`
	Grams               int     `json:"grams"               dynamodbav:"grams"`
	ImageID             string  `json:"imageId"             dynamodbav:"imageId"`
}

// Image holds a product image reference with optional variant association.
type Image struct {
	ImageID    string   `json:"imageId"    dynamodbav:"imageId"`
	URL        string   `json:"url"        dynamodbav:"url"`
	Name       string   `json:"name"       dynamodbav:"name"`
	AltText    string   `json:"altText"    dynamodbav:"altText"`
	Position   int      `json:"position"   dynamodbav:"position"`
	Width      int      `json:"width"      dynamodbav:"width"`
	Height     int      `json:"height"     dynamodbav:"height"`
	VariantIDs []string `json:"variantIds" dynamodbav:"variantIds"`
}

// CreateRequest is the expected JSON body for creating a product.
type CreateRequest struct {
	Handle         string      `json:"handle"`
	Title          string      `json:"title"`
	BodyHTML       string      `json:"bodyHtml"`
	Vendor         string      `json:"vendor"`
	ProductType    string      `json:"productType"`
	Tags           []string    `json:"tags"`
	Status         string      `json:"status"`
	Published      *bool       `json:"published,omitempty"`
	PublishedScope string      `json:"publishedScope,omitempty"`
	TemplateSuffix string      `json:"templateSuffix,omitempty"`
	SEOTitle       string      `json:"seoTitle,omitempty"`
	SEODescription string      `json:"seoDescription,omitempty"`
	GiftCard       *bool       `json:"giftCard,omitempty"`
	Metafields     []Metafield `json:"metafields,omitempty"`
	Options        []Option    `json:"options"`
	Variants       []Variant   `json:"variants"`
}

// UpdateRequest is the expected JSON body for updating a product.
type UpdateRequest struct {
	Handle         string      `json:"handle"`
	Title          string      `json:"title"`
	BodyHTML       string      `json:"bodyHtml"`
	Vendor         string      `json:"vendor"`
	ProductType    string      `json:"productType"`
	Tags           []string    `json:"tags"`
	Status         string      `json:"status"`
	Published      *bool       `json:"published,omitempty"`
	PublishedScope string      `json:"publishedScope,omitempty"`
	TemplateSuffix string      `json:"templateSuffix,omitempty"`
	SEOTitle       string      `json:"seoTitle,omitempty"`
	SEODescription string      `json:"seoDescription,omitempty"`
	GiftCard       *bool       `json:"giftCard,omitempty"`
	Metafields     []Metafield `json:"metafields,omitempty"`
	Options        []Option    `json:"options"`
	Variants       []Variant   `json:"variants"`
}

// BatchUpdateItem represents a partial update for a single product in a batch operation.
// All fields are pointers so nil means "do not change". ProductID is required.
type BatchUpdateItem struct {
	ProductID      string               `json:"productId"`
	Handle         *string              `json:"handle,omitempty"`
	Title          *string              `json:"title,omitempty"`
	BodyHTML       *string              `json:"bodyHtml,omitempty"`
	Vendor         *string              `json:"vendor,omitempty"`
	ProductType    *string              `json:"productType,omitempty"`
	Tags           *[]string            `json:"tags,omitempty"`
	Status         *string              `json:"status,omitempty"`
	Published      *bool                `json:"published,omitempty"`
	PublishedScope *string              `json:"publishedScope,omitempty"`
	TemplateSuffix *string              `json:"templateSuffix,omitempty"`
	SEOTitle       *string              `json:"seoTitle,omitempty"`
	SEODescription *string              `json:"seoDescription,omitempty"`
	GiftCard       *bool                `json:"giftCard,omitempty"`
	Metafields     *[]Metafield         `json:"metafields,omitempty"`
	Options        *[]Option            `json:"options,omitempty"`
	Variants       *[]BatchUpdateVariant `json:"variants,omitempty"`
}

// BatchUpdateVariant represents a partial update for a single variant.
// VariantID is required; all other fields are pointers so nil means "do not change".
type BatchUpdateVariant struct {
	VariantID           string   `json:"variantId"`
	Title               *string  `json:"title,omitempty"`
	SKU                 *string  `json:"sku,omitempty"`
	Barcode             *string  `json:"barcode,omitempty"`
	Price               *string  `json:"price,omitempty"`
	CompareAtPrice      *string  `json:"compareAtPrice,omitempty"`
	CostPerItem         *string  `json:"costPerItem,omitempty"`
	Position            *int     `json:"position,omitempty"`
	InventoryQty        *int     `json:"inventoryQty,omitempty"`
	InventoryPolicy     *string  `json:"inventoryPolicy,omitempty"`
	InventoryManagement *string  `json:"inventoryManagement,omitempty"`
	FulfillmentService  *string  `json:"fulfillmentService,omitempty"`
	Option1             *string  `json:"option1,omitempty"`
	Option2             *string  `json:"option2,omitempty"`
	Option3             *string  `json:"option3,omitempty"`
	Taxable             *bool    `json:"taxable,omitempty"`
	TaxCode             *string  `json:"taxCode,omitempty"`
	RequiresShipping    *bool    `json:"requiresShipping,omitempty"`
	Weight              *float64 `json:"weight,omitempty"`
	WeightUnit          *string  `json:"weightUnit,omitempty"`
	Grams               *int     `json:"grams,omitempty"`
	ImageID             *string  `json:"imageId,omitempty"`
}

// BatchUpdateResult summarizes the outcome of a batch product update.
type BatchUpdateResult struct {
	Updated []string           `json:"updated"`
	Failed  []BatchUpdateError `json:"failed"`
}

// BatchUpdateError describes a single product that failed during batch update.
type BatchUpdateError struct {
	ProductID string `json:"productId"`
	Message   string `json:"message"`
}

// ListResponse wraps a paginated list of products with bidirectional cursors.
type ListResponse struct {
	Products   []Product `json:"products"`
	NextCursor string    `json:"nextCursor,omitempty"`
	PrevCursor string    `json:"prevCursor,omitempty"`
}

// ListResult is the internal result type for paginated product queries,
// carrying products and cursor state between the repository and handler layers.
type ListResult struct {
	Products   []Product
	NextCursor string
	PrevCursor string
}

// ImportResult summarizes the outcome of a bulk product import.
type ImportResult struct {
	Imported int               `json:"imported"`
	Skipped  int               `json:"skipped"`
	Errors   []ValidationError `json:"errors"`
}

// ValidationError describes a single validation failure tied to a specific row and column.
type ValidationError struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
}

// SyncResult summarizes the outcome of a product sync operation.
type SyncResult struct {
	Created int         `json:"created"`
	Updated int         `json:"updated"`
	Failed  int         `json:"failed"`
	Errors  []SyncError `json:"errors,omitempty"`
}

// ResolveConflictRequest is the expected JSON body for resolving a sync conflict.
type ResolveConflictRequest struct {
	Resolution string `json:"resolution"`
}

// SyncError describes a single product that failed to sync.
type SyncError struct {
	ProductID string `json:"productId"`
	Message   string `json:"message"`
}

// UploadURLRequest is the expected JSON body for requesting a presigned upload URL.
type UploadURLRequest struct {
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
}

// UploadURLResponse contains the presigned URL and image key for client-side upload.
type UploadURLResponse struct {
	UploadURL string `json:"uploadUrl"`
	ImageKey  string `json:"imageKey"`
}

// ConfirmImageRequest is the expected JSON body for confirming an image upload.
type ConfirmImageRequest struct {
	ImageKey string `json:"imageKey"`
	Name     string `json:"name"`
	AltText  string `json:"altText"`
}

// UpdateImageRequest is the expected JSON body for updating image properties.
type UpdateImageRequest struct {
	Name     string `json:"name"`
	AltText  string `json:"altText"`
	Position int    `json:"position"`
}

// ReorderImageEntry specifies a new position for a single image.
type ReorderImageEntry struct {
	ImageID  string `json:"imageId"`
	Position int    `json:"position"`
}

// ShopifyImageOp groups the parameters for a Shopify image operation.
type ShopifyImageOp struct {
	UserID  string
	StoreID string
	Product *Product
}

// ProductSyncer defines the sync operations used by the product handler.
type ProductSyncer interface {
	PullFromShopify(ctx context.Context, userID, storeID string) (*SyncResult, error)
	PushAllToShopify(ctx context.Context, userID, storeID string) (*SyncResult, error)
	PushToShopify(ctx context.Context, userID, storeID, productID string) error
	FetchShopifyVersion(ctx context.Context, userID, storeID, productID string) (*Product, error)
	ResolveKeepLocal(ctx context.Context, userID, storeID, productID string) error
	ResolveUseShopify(ctx context.Context, userID, storeID, productID string) error
}
