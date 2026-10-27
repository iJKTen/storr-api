package integration

import "time"

// Shopify OAuth and API constants.
const (
	// ShopifyOAuthScopes defines the permissions requested during Shopify OAuth authorization.
	ShopifyOAuthScopes = "read_products,write_products,read_publications,write_publications,read_metafield_definitions,unauthenticated_read_product_listings"

	// ShopifyStorefrontTokenTitle is the display title for the storefront access token created via GraphQL.
	ShopifyStorefrontTokenTitle = "storr-pipeline"

	// ShopifyDomainPattern validates that a shop domain is a legitimate myshopify.com subdomain.
	// Prevents OAuth redirects to attacker-controlled hosts.
	ShopifyDomainPattern = `^[a-zA-Z0-9][a-zA-Z0-9\-]*\.myshopify\.com$`
)

// Figma OAuth and API constants.
const (
	// FigmaOAuthScopes defines the permissions requested during Figma OAuth authorization.
	FigmaOAuthScopes = "file_content:read,file_metadata:read,file_versions:read,library_assets:read,library_content:read"

	// FigmaOAuthBaseURL is the base URL for Figma's OAuth authorization page.
	FigmaOAuthBaseURL = "https://www.figma.com/oauth?"

	// FigmaTokenEndpoint is the URL for exchanging an authorization code for tokens.
	FigmaTokenEndpoint = "https://api.figma.com/v1/oauth/token"

	// FigmaRefreshEndpoint is the URL for refreshing an expired Figma access token.
	FigmaRefreshEndpoint = "https://api.figma.com/v1/oauth/refresh"
)

// FigmaTokenRefreshThreshold is how far before expiry we proactively refresh the Figma token.
// Tokens are refreshed if they expire within this window to avoid mid-operation failures.
var FigmaTokenRefreshThreshold = 7 * 24 * time.Hour

// OAuthStateTTL is how long an OAuth state token remains valid before expiring.
// Limits the window for CSRF attacks via stale state parameters.
var OAuthStateTTL = 5 * time.Minute

// Integration origin constants.
const (
	// IntegrationOriginOAuth marks an integration as connected via OAuth flow.
	IntegrationOriginOAuth = "oauth"
)
