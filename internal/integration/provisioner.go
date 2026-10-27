package integration

import (
	"context"
	"log/slog"
	"strings"

	"storr-api/internal/secrets"
	"storr-api/internal/store"

	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// PipelineEnvConfig holds LLM provider credentials injected into pipeline containers.
type PipelineEnvConfig struct {
	AnthropicKey string
	LlmModel     string
	LlmProvider  string
}

// IntegrationClients groups the service clients needed by the Provisioner.
type IntegrationClients struct {
	Secrets    *secrets.SecretsClient
	FigmaOAuth *FigmaOAuthService
}

// Provisioner builds environment variable overrides for pipeline containers.
type Provisioner struct {
	clients     IntegrationClients
	pipelineEnv PipelineEnvConfig
}

// NewProvisioner creates a Provisioner with the given integration clients and env config.
func NewProvisioner(clients IntegrationClients, pipelineEnv PipelineEnvConfig) *Provisioner {
	return &Provisioner{
		clients:     clients,
		pipelineEnv: pipelineEnv,
	}
}

// BuildEnvOverrides assembles ECS environment overrides from integration secrets and config.
func (p *Provisioner) BuildEnvOverrides(ctx context.Context, pc store.ProvisionContext) ([]types.KeyValuePair, error) {
	var envVars []types.KeyValuePair
	if len(pc.FigmaFileKeys) > 0 {
		envVars = append(envVars, kv("FIGMA_FILE_KEYS", strings.Join(pc.FigmaFileKeys, ",")))
	}
	envVars = append(envVars, p.buildFigmaEnv(ctx, pc)...)
	envVars = append(envVars, p.buildShopifyEnv(ctx, pc)...)
	envVars = append(envVars, p.buildPipelineEnv()...)
	return envVars, nil
}

func (p *Provisioner) buildPipelineEnv() []types.KeyValuePair {
	var envVars []types.KeyValuePair
	if p.pipelineEnv.AnthropicKey != "" {
		envVars = append(envVars, kv("ANTHROPIC_API_KEY", p.pipelineEnv.AnthropicKey))
	}
	if p.pipelineEnv.LlmProvider != "" {
		envVars = append(envVars, kv("LLM_PROVIDER", p.pipelineEnv.LlmProvider))
	}
	if p.pipelineEnv.LlmModel != "" {
		modelKey := "OPENAI_MODEL"
		if p.pipelineEnv.LlmProvider == "anthropic" {
			modelKey = "ANTHROPIC_MODEL"
		}
		envVars = append(envVars, kv(modelKey, p.pipelineEnv.LlmModel))
	}
	return envVars
}

func (p *Provisioner) buildFigmaEnv(ctx context.Context, pc store.ProvisionContext) []types.KeyValuePair {
	cfg, ok := pc.Integrations[store.ProviderFigma]
	if !ok || !cfg.Enabled {
		return nil
	}
	secretName := secrets.FigmaSecretName(pc.UserID, pc.StoreID)
	token := p.resolveFigmaToken(ctx, secretName, pc.UserID)
	if token == "" {
		return nil
	}
	return []types.KeyValuePair{kv("FIGMA_OAUTH_TOKEN", token)}
}

// resolveFigmaToken loads and optionally refreshes the Figma access token.
// Errors are logged but not returned because the pipeline can proceed with a
// stale token; the Figma API will reject it and the user can re-authorize.
func (p *Provisioner) resolveFigmaToken(ctx context.Context, secretName, customerID string) string {
	figmaSecret, err := secrets.GetSecret[secrets.FigmaSecret](p.clients.Secrets, ctx, secretName)
	if err != nil {
		slog.Warn("failed to get figma secret", "customerId", customerID, "error", err)
		return ""
	}
	refreshed, err := p.clients.FigmaOAuth.RefreshIfNeeded(ctx, figmaSecret)
	if err != nil {
		slog.Warn("failed to refresh figma token", "customerId", customerID, "error", err)
		return figmaSecret.AccessToken
	}
	if refreshed.AccessToken != figmaSecret.AccessToken {
		p.updateFigmaSecret(ctx, secretName, refreshed)
	}
	return refreshed.AccessToken
}

func (p *Provisioner) updateFigmaSecret(ctx context.Context, secretName string, secret *secrets.FigmaSecret) {
	if _, err := secrets.PutSecret(p.clients.Secrets, ctx, secretName, *secret); err != nil {
		slog.Warn("failed to update refreshed figma token", "error", err)
	}
}

func (p *Provisioner) buildShopifyEnv(ctx context.Context, pc store.ProvisionContext) []types.KeyValuePair {
	cfg, ok := pc.Integrations[store.ProviderShopify]
	if !ok || !cfg.Enabled {
		return nil
	}
	return p.fetchShopifySecrets(ctx, pc.UserID, pc.StoreID)
}

func (p *Provisioner) fetchShopifySecrets(ctx context.Context, customerID, storeID string) []types.KeyValuePair {
	secretName := secrets.ShopifySecretName(customerID, storeID)
	secret, err := secrets.GetSecret[secrets.ShopifySecret](p.clients.Secrets, ctx, secretName)
	if err != nil {
		slog.Warn("failed to get shopify secret", "storeId", storeID, "error", err)
		return nil
	}
	return []types.KeyValuePair{
		kv("PUBLIC_STORE_DOMAIN", secret.StoreDomain),
		kv("PUBLIC_STOREFRONT_API_TOKEN", secret.StorefrontToken),
		kv("PUBLIC_STOREFRONT_API_VERSION", secret.APIVersion),
	}
}

func kv(name, value string) types.KeyValuePair {
	return types.KeyValuePair{Name: &name, Value: &value}
}
