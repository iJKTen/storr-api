package integration

// Integration represents a provider integration record in DynamoDB.
type Integration struct {
	CustomerID     string `json:"customerId"     dynamodbav:"customerId"`
	IntegrationKey string `json:"integrationKey" dynamodbav:"integrationKey"`
	Status         string `json:"status"         dynamodbav:"status"`
	SecretArn      string `json:"secretArn"      dynamodbav:"secretArn"`
}

// StartFigmaResponse is the API response containing the Figma OAuth authorization URL.
type StartFigmaResponse struct {
	AuthURL string `json:"authUrl"`
	State   string `json:"state"`
}

// StartShopifyRequest is the expected JSON body for initiating Shopify OAuth.
type StartShopifyRequest struct {
	Shop string `json:"shop"`
}

// StartShopifyResponse is the API response containing the Shopify install URL.
type StartShopifyResponse struct {
	InstallURL string `json:"installUrl"`
	State      string `json:"state"`
}
