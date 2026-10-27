package sync

import (
	"storr-api/internal/product"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ShopifyCreds holds the admin token and store domain for Shopify API calls.
type ShopifyCreds struct {
	AdminToken string
	Domain     string
}

// ShopifyProductPage holds a page of products fetched from Shopify, converted to Storr format.
type ShopifyProductPage struct {
	Products []product.Product
	Cursor   string
	HasNext  bool
}

type metafieldInput struct {
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	Type      string `json:"type"`
}

type seoInput struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type inventoryItemInput struct {
	Cost string `json:"cost,omitempty"`
}

// ProductSetInput is the input for the Shopify productSet mutation.
type ProductSetInput struct {
	ID              string            `json:"id,omitempty"`
	Handle          string            `json:"handle"`
	Title           string            `json:"title"`
	DescriptionHTML string            `json:"descriptionHtml,omitempty"`
	Vendor          string            `json:"vendor,omitempty"`
	ProductType     string            `json:"productType,omitempty"`
	Tags            []string          `json:"tags,omitempty"`
	Status          string            `json:"status"`
	TemplateSuffix  string            `json:"templateSuffix,omitempty"`
	GiftCard        *bool             `json:"giftCard,omitempty"`
	SEO             *seoInput         `json:"seo,omitempty"`
	Metafields      []metafieldInput  `json:"metafields,omitempty"`
	ProductOptions  []OptionSetInput  `json:"productOptions,omitempty"`
	Variants        []VariantSetInput `json:"variants,omitempty"`
}

// OptionSetInput defines a product option for the Shopify productSet mutation.
type OptionSetInput struct {
	Name   string        `json:"name"`
	Values []OptionValue `json:"values"`
}

// OptionValue holds a single option value name.
type OptionValue struct {
	Name string `json:"name"`
}

// VariantSetInput defines a variant for the Shopify productSet mutation.
type VariantSetInput struct {
	ID               string             `json:"id,omitempty"`
	SKU              string             `json:"sku,omitempty"`
	Barcode          string             `json:"barcode,omitempty"`
	Price            string             `json:"price"`
	CompareAtPrice   string             `json:"compareAtPrice,omitempty"`
	Position         int                `json:"position,omitempty"`
	InventoryPolicy  string             `json:"inventoryPolicy,omitempty"`
	Taxable          bool               `json:"taxable"`
	TaxCode          string             `json:"taxCode,omitempty"`
	RequiresShipping bool               `json:"requiresShipping"`
	Weight           float64            `json:"weight,omitempty"`
	WeightUnit       string             `json:"weightUnit,omitempty"`
	InventoryItem    *inventoryItemInput `json:"inventoryItem,omitempty"`
	OptionValues     []OptionValueRef   `json:"optionValues,omitempty"`
}

// OptionValueRef links a variant to an option value by name.
type OptionValueRef struct {
	OptionName string `json:"optionName"`
	Name       string `json:"name"`
}

// StagedUploadTarget holds the URL and parameters returned by Shopify's stagedUploadsCreate mutation.
type StagedUploadTarget struct {
	URL         string
	ResourceURL string
	Parameters  []StagedUploadParam
}

// StagedUploadParam is a key-value pair for staged upload multipart form parameters.
type StagedUploadParam struct {
	Name  string
	Value string
}

// MediaInput describes a media item to associate with a Shopify product.
type MediaInput struct {
	OriginalSource   string
	Alt              string
	MediaContentType string
}

// MoveInput describes a media reorder operation for Shopify's productReorderMedia mutation.
type MoveInput struct {
	ID          string `json:"id"`
	NewPosition string `json:"newPosition"`
}

// ImageSyncDeps holds S3 dependencies for syncing product images with Shopify.
type ImageSyncDeps struct {
	Presign *s3.PresignClient
	S3      *s3.Client
	Bucket  string
}

// --- GraphQL Variable Types ---

type fetchProductsVars struct {
	First int    `json:"first"`
	After string `json:"after,omitempty"`
}

type fetchProductByIDVars struct {
	ID string `json:"id"`
}

type upsertProductVars struct {
	Input       ProductSetInput `json:"input"`
	Synchronous bool            `json:"synchronous"`
}

// --- GraphQL Response Types ---

type shopifyGQLError struct {
	Message string `json:"message"`
}

type productsGQLResponse struct {
	Data   productsData      `json:"data"`
	Errors []shopifyGQLError `json:"errors"`
}

type nodeGQLResponse struct {
	Data   nodeData          `json:"data"`
	Errors []shopifyGQLError `json:"errors"`
}

type nodeData struct {
	Node productNode `json:"node"`
}

type productsData struct {
	Products productsConn `json:"products"`
}

type productsConn struct {
	Edges    []productEdge `json:"edges"`
	PageInfo pageInfo      `json:"pageInfo"`
}

type productEdge struct {
	Node productNode `json:"node"`
}

type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type productNodeSEO struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type productNodeMFConn struct {
	Nodes []productNodeMF `json:"nodes"`
}

type productNodeMF struct {
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	Type      string `json:"type"`
}

type productNode struct {
	ID              string            `json:"id"`
	Handle          string            `json:"handle"`
	Title           string            `json:"title"`
	DescriptionHTML string            `json:"descriptionHtml"`
	Vendor          string            `json:"vendor"`
	ProductType     string            `json:"productType"`
	Tags            []string          `json:"tags"`
	Status          string            `json:"status"`
	PublishedAt     *string           `json:"publishedAt"`
	TemplateSuffix  string            `json:"templateSuffix"`
	IsGiftCard      bool              `json:"isGiftCard"`
	SEO             productNodeSEO    `json:"seo"`
	Metafields      productNodeMFConn `json:"metafields"`
	Options         []optionNode      `json:"options"`
	Variants        variantConn       `json:"variants"`
	Images          imageConn         `json:"images"`
}

type optionNode struct {
	Name     string   `json:"name"`
	Position int      `json:"position"`
	Values   []string `json:"values"`
}

type variantConn struct {
	Edges []variantEdge `json:"edges"`
}

type variantEdge struct {
	Node variantNode `json:"node"`
}

type variantNodeInvItem struct {
	UnitCost *variantNodeMoney `json:"unitCost"`
}

type variantNodeMoney struct {
	Amount string `json:"amount"`
}

type variantNode struct {
	ID                string           `json:"id"`
	Title             string           `json:"title"`
	SKU               string           `json:"sku"`
	Barcode           string           `json:"barcode"`
	Price             string           `json:"price"`
	CompareAtPrice    *string          `json:"compareAtPrice"`
	Position          int              `json:"position"`
	InventoryQuantity int              `json:"inventoryQuantity"`
	InventoryPolicy   string           `json:"inventoryPolicy"`
	Taxable           bool             `json:"taxable"`
	TaxCode           string           `json:"taxCode"`
	RequiresShipping  bool             `json:"requiresShipping"`
	Weight            float64          `json:"weight"`
	WeightUnit        string           `json:"weightUnit"`
	InventoryItem     variantNodeInvItem `json:"inventoryItem"`
	SelectedOptions   []selectedOption `json:"selectedOptions"`
	Image             *imageRef        `json:"image"`
}

type selectedOption struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type imageRef struct {
	ID string `json:"id"`
}

type imageConn struct {
	Edges []imageEdge `json:"edges"`
}

type imageEdge struct {
	Node imageNode `json:"node"`
}

type imageNode struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	AltText string `json:"altText"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

type productSetGQLResponse struct {
	Data   productSetData    `json:"data"`
	Errors []shopifyGQLError `json:"errors"`
}

type productSetData struct {
	ProductSet productSetResult `json:"productSet"`
}

type productSetResult struct {
	Product    *productIDRef   `json:"product"`
	UserErrors []shopifyUError `json:"userErrors"`
}

type productIDRef struct {
	ID string `json:"id"`
}

type shopifyUError struct {
	Field   []string `json:"field"`
	Message string   `json:"message"`
}

// --- Media GQL Variable Types ---

type stagedUploadVars struct {
	Input []stagedUploadInput `json:"input"`
}

type stagedUploadInput struct {
	Resource   string `json:"resource"`
	Filename   string `json:"filename"`
	MimeType   string `json:"mimeType"`
	HttpMethod string `json:"httpMethod"`
}

type createMediaVars struct {
	ProductID string                `json:"productId"`
	Media     []gqlCreateMediaInput `json:"media"`
}

type gqlCreateMediaInput struct {
	OriginalSource   string `json:"originalSource"`
	Alt              string `json:"alt"`
	MediaContentType string `json:"mediaContentType"`
}

type deleteMediaVars struct {
	ProductID string   `json:"productId"`
	MediaIDs  []string `json:"mediaIds"`
}

type reorderMediaVars struct {
	ID    string      `json:"id"`
	Moves []MoveInput `json:"moves"`
}

// --- Media GQL Response Types ---

type stagedUploadGQLResponse struct {
	Data   stagedUploadData  `json:"data"`
	Errors []shopifyGQLError `json:"errors"`
}

type stagedUploadData struct {
	StagedUploadsCreate stagedUploadResultData `json:"stagedUploadsCreate"`
}

type stagedUploadResultData struct {
	StagedTargets []gqlStagedTarget `json:"stagedTargets"`
	UserErrors    []shopifyUError   `json:"userErrors"`
}

type gqlStagedTarget struct {
	URL         string           `json:"url"`
	ResourceURL string           `json:"resourceUrl"`
	Parameters  []gqlStagedParam `json:"parameters"`
}

type gqlStagedParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type createMediaGQLResponse struct {
	Data   createMediaData   `json:"data"`
	Errors []shopifyGQLError `json:"errors"`
}

type createMediaData struct {
	ProductCreateMedia createMediaResultData `json:"productCreateMedia"`
}

type createMediaResultData struct {
	MediaUserErrors []shopifyUError `json:"mediaUserErrors"`
}

type deleteMediaGQLResponse struct {
	Data   deleteMediaData   `json:"data"`
	Errors []shopifyGQLError `json:"errors"`
}

type deleteMediaData struct {
	ProductDeleteMedia deleteMediaResultData `json:"productDeleteMedia"`
}

type deleteMediaResultData struct {
	DeletedMediaIDs []string        `json:"deletedMediaIds"`
	MediaUserErrors []shopifyUError `json:"mediaUserErrors"`
}

type reorderMediaGQLResponse struct {
	Data   reorderMediaData  `json:"data"`
	Errors []shopifyGQLError `json:"errors"`
}

type reorderMediaData struct {
	ProductReorderMedia reorderMediaResultData `json:"productReorderMedia"`
}

type reorderMediaResultData struct {
	Job             *jobRef         `json:"job"`
	MediaUserErrors []shopifyUError `json:"mediaUserErrors"`
}

type jobRef struct {
	ID string `json:"id"`
}

// --- Publication GQL Types ---

type publicationsQueryVars struct {
	First int `json:"first"`
}

type publicationVars struct {
	ID    string             `json:"id"`
	Input []publicationInput `json:"input"`
}

type publicationInput struct {
	PublicationID string `json:"publicationId"`
}

type publicationsGQLResponse struct {
	Data   publicationsData  `json:"data"`
	Errors []shopifyGQLError `json:"errors"`
}

type publicationsData struct {
	Publications publicationsConn `json:"publications"`
}

type publicationsConn struct {
	Nodes []publicationNode `json:"nodes"`
}

type publicationNode struct {
	ID      string             `json:"id"`
	Catalog publicationCatalog `json:"catalog"`
}

type publicationCatalog struct {
	Title string `json:"title"`
}

type publishGQLResponse struct {
	Data   publishData       `json:"data"`
	Errors []shopifyGQLError `json:"errors"`
}

type publishData struct {
	Publish   *publishResult `json:"publishablePublish,omitempty"`
	Unpublish *publishResult `json:"publishableUnpublish,omitempty"`
}

type publishResult struct {
	UserErrors []shopifyUError `json:"userErrors"`
}
