package product

const (
	// MaxVariantsPerProduct is the Shopify limit on variants per product.
	MaxVariantsPerProduct = 100

	// MaxImagesPerProduct is the Shopify limit on images per product.
	MaxImagesPerProduct = 250

	// MaxOptionsPerProduct is the Shopify limit on option axes per product.
	MaxOptionsPerProduct = 3

	// BatchWriteChunkSize is the DynamoDB BatchWriteItem limit per request.
	BatchWriteChunkSize = 25

	// DefaultPageSize is the default number of products returned per list request.
	DefaultPageSize = 50

	// MaxPageSize is the maximum allowed page size for list requests.
	MaxPageSize = 200

	// DefaultInventoryPolicy controls selling behavior when inventory reaches zero.
	DefaultInventoryPolicy = "deny"

	// DefaultFulfillmentService is the default fulfillment method for new variants.
	DefaultFulfillmentService = "manual"

	// DefaultWeightUnit is the default unit for variant weight when not specified.
	DefaultWeightUnit = "lb"

	// MaxImportFileBytes is the maximum allowed size for product import files (10 MB).
	MaxImportFileBytes = 10 << 20

	// TemplateXLSXFilename is the download filename for the Excel product template.
	TemplateXLSXFilename = "storr-product-template.xlsx"

	// TemplateCSVFilename is the download filename for the CSV product template.
	TemplateCSVFilename = "storr-product-template.csv"

	// WebhookEchoWindowSeconds is the time window to detect echo webhooks from our own pushes.
	WebhookEchoWindowSeconds = 30

	// WebhookEventTTLHours is how long webhook event IDs are kept for deduplication.
	WebhookEventTTLHours = 24

	// PresignedURLExpirySec is how long a presigned upload URL remains valid (5 minutes).
	PresignedURLExpirySec = 300

	// WebhookMaxBodyBytes is the maximum allowed webhook request body size (5 MB).
	// Prevents memory exhaustion on the unauthenticated webhook endpoint.
	WebhookMaxBodyBytes = 5 << 20

	// ImageKeyPrefix is the S3 key prefix for product images.
	ImageKeyPrefix = "stores"

	// ShopifyProductGIDFmt is the GID format for Shopify product IDs.
	ShopifyProductGIDFmt = "gid://shopify/Product/%d"

	// ShopifyVariantGIDFmt is the GID format for Shopify variant IDs.
	ShopifyVariantGIDFmt = "gid://shopify/ProductVariant/%d"

	// ShopifyImageGIDFmt is the GID format for Shopify image IDs.
	ShopifyImageGIDFmt = "gid://shopify/ProductImage/%d"

	// ResolutionKeepLocal is the conflict resolution strategy that keeps the local version.
	ResolutionKeepLocal = "keep_local"

	// ResolutionUseShopify is the conflict resolution strategy that uses the Shopify version.
	ResolutionUseShopify = "use_shopify"

	// ProductIDPrefix is the prefix for generated product IDs.
	ProductIDPrefix = "prod-"

	// VariantIDPrefix is the prefix for generated variant IDs.
	VariantIDPrefix = "var-"

	// ImageIDPrefix is the prefix for generated image IDs.
	ImageIDPrefix = "img-"

	// DefaultMetafieldType is the default metafield type used when importing from CSV.
	DefaultMetafieldType = "single_line_text_field"

	// importErrSaveFailed is the sanitized message returned to clients
	// when a product fails to persist during import. The raw error is
	// logged internally and never exposed.
	importErrSaveFailed = "failed to save product"

	// batchUpdateErrGeneric is the sanitized message returned to clients
	// when a product fails to update during a batch operation due to
	// infrastructure errors. The raw error is logged internally.
	batchUpdateErrGeneric = "failed to update product"

	// BatchGetChunkSize is the DynamoDB BatchGetItem limit per request.
	BatchGetChunkSize = 100

	// ShopifyMediaGIDPrefix is the prefix for Shopify media global IDs.
	ShopifyMediaGIDPrefix = "gid://shopify/"
)

// PageDirection indicates the direction of cursor-based pagination.
type PageDirection string

const (
	// PageForward paginates forward (ascending sort key order).
	PageForward PageDirection = "forward"

	// PageBackward paginates backward (descending sort key order).
	PageBackward PageDirection = "backward"
)
