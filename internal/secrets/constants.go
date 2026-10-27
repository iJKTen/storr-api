package secrets

// ShopifySecretNameFmt is the Secrets Manager path template for Shopify credentials.
// Format: storr/{customerID}/shopify/{storeID}
const ShopifySecretNameFmt = "storr/%s/shopify/%s"

// FigmaSecretNameFmt is the Secrets Manager path template for Figma credentials.
// Format: storr/{customerID}/figma/{storeID}
const FigmaSecretNameFmt = "storr/%s/figma/%s"

// ShopifyAPIVersion is the Shopify Admin API version used for all GraphQL and REST calls.
const ShopifyAPIVersion = "2025-01"
