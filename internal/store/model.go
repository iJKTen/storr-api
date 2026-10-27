package store

// ProjectType indicates the store's build pipeline type.
type ProjectType string

const (
	ProjectTypeHydrogen ProjectType = "hydrogen"
	ProjectTypeTheme    ProjectType = "theme"

	StoreIDPrefix = "store-"

	// ShopifyURLIndexName is the DynamoDB GSI name for looking up stores by Shopify URL.
	ShopifyURLIndexName = "shopifyUrl-index"
)

// Store represents a user's store project in DynamoDB.
type Store struct {
	ID            string                       `json:"id"            dynamodbav:"storeId"`
	UserID        string                       `json:"-"             dynamodbav:"userId"`
	Name          string                       `json:"name"          dynamodbav:"name"`
	ProjectType   ProjectType                  `json:"projectType"   dynamodbav:"projectType"`
	ShopifyURL    string                       `json:"shopifyUrl"    dynamodbav:"shopifyUrl"`
	FigmaFileKeys []string                     `json:"figmaFileKeys" dynamodbav:"figmaFileKeys"`
	Integrations  map[string]IntegrationConfig `json:"integrations,omitempty" dynamodbav:"integrations,omitempty"`
	CreatedAt     string                       `json:"createdAt"     dynamodbav:"createdAt"`
}

// IntegrationConfig holds the enabled state and settings for a single integration.
type IntegrationConfig struct {
	Enabled bool              `json:"enabled" dynamodbav:"enabled"`
	Values  map[string]string `json:"values"  dynamodbav:"values"`
	Origin  string            `json:"origin"  dynamodbav:"origin"`
}

// CreateStoreRequest is the expected JSON body for creating a new store.
type CreateStoreRequest struct {
	Name        string   `json:"name"`
	ProjectType string   `json:"projectType"`
	ShopifyURL  string   `json:"shopifyUrl"`
	FigmaURLs   []string `json:"figmaUrls"`
}

// UpdateStoreRequest is the expected JSON body for updating a store.
type UpdateStoreRequest struct {
	Name       string   `json:"name"`
	ShopifyURL string   `json:"shopifyUrl"`
	FigmaURLs  []string `json:"figmaUrls"`
}

// IntegrationUpdate pairs an integration key with its new configuration.
type IntegrationUpdate struct {
	Key    string
	Config IntegrationConfig
}

// PipelineContext identifies the build pipeline routing configuration for a store.
type PipelineContext struct {
	StoreID     string
	ProjectType ProjectType
	ShopifyURL  string
}

// ProvisionContext contains credentials and integration data needed for pipeline provisioning.
type ProvisionContext struct {
	UserID        string
	StoreID       string
	FigmaFileKeys []string
	Integrations  map[string]IntegrationConfig
}

// PipelineContext returns the pipeline routing context derived from this store.
func (s *Store) PipelineContext() PipelineContext {
	return PipelineContext{
		StoreID:     s.ID,
		ProjectType: s.ProjectType,
		ShopifyURL:  s.ShopifyURL,
	}
}

// ProvisionContext returns the provisioning context derived from this store.
func (s *Store) ProvisionContext() ProvisionContext {
	return ProvisionContext{
		UserID:        s.UserID,
		StoreID:       s.ID,
		FigmaFileKeys: s.FigmaFileKeys,
		Integrations:  s.Integrations,
	}
}
