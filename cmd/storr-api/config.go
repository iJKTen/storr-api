package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"storr-api/internal/infra"
	"storr-api/internal/integration"
	"storr-api/internal/pipeline"
)

type corsConfig struct {
	Origin string
}

type tablesConfig struct {
	Stores        string
	OAuthStates   string
	Products      string
	WebhookEvents string
}

type bucketsConfig struct {
	ProductImages string
}

type productConfig struct {
	BatchUpdateMaxProducts int
}

type appConfig struct {
	CORS        corsConfig
	Tables      tablesConfig
	Buckets     bucketsConfig
	Products    productConfig
	FrontendURL string
}

func (c appConfig) validate() error {
	var missing []string
	if c.Tables.Stores == "" {
		missing = append(missing, "STORES_TABLE")
	}
	if c.Tables.OAuthStates == "" {
		missing = append(missing, "OAUTH_STATES_TABLE")
	}
	if c.Tables.Products == "" {
		missing = append(missing, "PRODUCTS_TABLE")
	}
	if c.Tables.WebhookEvents == "" {
		missing = append(missing, "WEBHOOK_EVENTS_TABLE")
	}
	if c.Buckets.ProductImages == "" {
		missing = append(missing, "PRODUCT_IMAGES_BUCKET")
	}
	if c.FrontendURL == "" {
		missing = append(missing, "FRONTEND_URL")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	return nil
}

func loadConfig() appConfig {
	return appConfig{
		CORS:        corsConfig{Origin: os.Getenv("CORS_ORIGIN")},
		Tables:      loadTablesConfig(),
		Buckets:     loadBucketsConfig(),
		Products:    loadProductConfig(),
		FrontendURL: os.Getenv("FRONTEND_URL"),
	}
}

func loadProductConfig() productConfig {
	maxProducts, _ := strconv.Atoi(os.Getenv("BATCH_UPDATE_MAX_PRODUCTS"))
	return productConfig{
		BatchUpdateMaxProducts: maxProducts,
	}
}

func loadBucketsConfig() bucketsConfig {
	return bucketsConfig{
		ProductImages: os.Getenv("PRODUCT_IMAGES_BUCKET"),
	}
}

func loadTablesConfig() tablesConfig {
	return tablesConfig{
		Stores:        os.Getenv("STORES_TABLE"),
		OAuthStates:   os.Getenv("OAUTH_STATES_TABLE"),
		Products:      os.Getenv("PRODUCTS_TABLE"),
		WebhookEvents: os.Getenv("WEBHOOK_EVENTS_TABLE"),
	}
}

type cognitoConfig struct {
	PoolID   string
	ClientID string
}

func loadCognitoConfig() cognitoConfig {
	return cognitoConfig{
		PoolID:   os.Getenv("COGNITO_USER_POOL_ID"),
		ClientID: os.Getenv("COGNITO_CLIENT_ID"),
	}
}

func loadThemeConfig() pipeline.ThemeConfig {
	cluster := os.Getenv("THEME_CLUSTER")
	if cluster == "" {
		cluster = os.Getenv("STUDIO_CLUSTER")
	}
	containerName := os.Getenv("THEME_CONTAINER_NAME")
	if containerName == "" {
		containerName = infra.StudioContainerName
	}
	return pipeline.ThemeConfig{
		ClusterName:    cluster,
		TaskDefinition: os.Getenv("THEME_TASK_DEFINITION"),
		ContainerName:  containerName,
		Network: pipeline.NetworkConfig{
			SubnetIDs:       splitNonEmpty(os.Getenv("STUDIO_SUBNETS"), ","),
			SecurityGroupID: os.Getenv("STUDIO_SECURITY_GROUP"),
			AlbListenerArn:  os.Getenv("STUDIO_ALB_LISTENER_ARN"),
			VpcID:           os.Getenv("STUDIO_VPC_ID"),
		},
	}
}

func loadPipelineConfig() pipeline.Config {
	cluster := os.Getenv("PIPELINE_CLUSTER")
	if cluster == "" {
		cluster = os.Getenv("STUDIO_CLUSTER")
	}
	return pipeline.Config{
		Task: pipeline.TaskConfig{
			ClusterName:    cluster,
			TaskDefinition: os.Getenv("PIPELINE_TASK_DEFINITION"),
		},
		Network: pipeline.NetworkConfig{
			SubnetIDs:       splitNonEmpty(os.Getenv("STUDIO_SUBNETS"), ","),
			SecurityGroupID: os.Getenv("STUDIO_SECURITY_GROUP"),
			AlbListenerArn:  os.Getenv("STUDIO_ALB_LISTENER_ARN"),
			VpcID:           os.Getenv("STUDIO_VPC_ID"),
		},
	}
}

func loadPreviewRouteConfig() infra.PreviewRouteConfig {
	return infra.PreviewRouteConfig{
		ListenerArn: os.Getenv("STUDIO_ALB_LISTENER_ARN"),
		VpcID:       os.Getenv("STUDIO_VPC_ID"),
	}
}

type pipelineEnvParams struct {
	AnthropicKeyParam string
	LlmModelParam     string
	LlmProviderParam  string
}

func loadPipelineEnvParams() pipelineEnvParams {
	return pipelineEnvParams{
		AnthropicKeyParam: os.Getenv("ANTHROPIC_API_KEY_PARAM"),
		LlmModelParam:     os.Getenv("LLM_MODEL_PARAM"),
		LlmProviderParam:  os.Getenv("LLM_PROVIDER_PARAM"),
	}
}

func loadFigmaOAuthConfig() integration.FigmaOAuthConfig {
	return integration.FigmaOAuthConfig{
		ClientID:     os.Getenv("FIGMA_CLIENT_ID"),
		ClientSecret: os.Getenv("FIGMA_CLIENT_SECRET"),
		RedirectURI:  os.Getenv("FIGMA_REDIRECT_URI"),
	}
}

func loadShopifyOAuthConfig() integration.ShopifyOAuthConfig {
	return integration.ShopifyOAuthConfig{
		ClientID:     os.Getenv("SHOPIFY_CLIENT_ID"),
		ClientSecret: os.Getenv("SHOPIFY_CLIENT_SECRET"),
		RedirectURI:  os.Getenv("SHOPIFY_REDIRECT_URI"),
	}
}

// splitNonEmpty splits s by sep and filters out empty strings.
// Prevents strings.Split("", ",") from returning [""].
func splitNonEmpty(s, sep string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
