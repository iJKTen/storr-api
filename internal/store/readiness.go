package store

// Provider name constants used in integration keys and OAuth state records.
const (
	// ProviderShopify identifies the Shopify integration provider.
	ProviderShopify = "shopify"

	// ProviderFigma identifies the Figma integration provider.
	ProviderFigma = "figma"
)

// IntegrationStatus reports whether a single integration provider is connected.
type IntegrationStatus struct {
	Connected bool `json:"connected"`
}

// ReadinessResponse summarizes whether all required integrations are connected.
type ReadinessResponse struct {
	Figma   IntegrationStatus `json:"figma"`
	Shopify IntegrationStatus `json:"shopify"`
	Ready   bool              `json:"ready"`
}

// CheckReadiness evaluates whether both Figma and Shopify integrations are enabled.
func CheckReadiness(integrations map[string]IntegrationConfig) *ReadinessResponse {
	figma := IntegrationStatus{Connected: isEnabled(integrations, ProviderFigma)}
	shopify := IntegrationStatus{Connected: isEnabled(integrations, ProviderShopify)}
	return &ReadinessResponse{
		Figma:   figma,
		Shopify: shopify,
		Ready:   figma.Connected && shopify.Connected,
	}
}

func isEnabled(integrations map[string]IntegrationConfig, key string) bool {
	cfg, ok := integrations[key]
	return ok && cfg.Enabled
}
