package secrets

// FigmaSecret is stored in Secrets Manager at storr/{customerId}/figma.
type FigmaSecret struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    string `json:"expiresAt"`
}

// ShopifySecret is stored in Secrets Manager at storr/{customerId}/shopify/{projectId}.
type ShopifySecret struct {
	AdminToken      string `json:"adminToken"`
	StorefrontToken string `json:"storefrontToken"`
	StoreDomain     string `json:"storeDomain"`
	APIVersion      string `json:"apiVersion"`
}
