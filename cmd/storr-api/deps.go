package main

import (
	"context"
	"log/slog"
	"os"

	"storr-api/internal/auth"
	"storr-api/internal/infra"
	"storr-api/internal/integration"
	"storr-api/internal/pipeline"
	"storr-api/internal/product"
	"storr-api/internal/secrets"
	"storr-api/internal/product/image"
	"storr-api/internal/product/importexport"
	"storr-api/internal/product/sync"
	"storr-api/internal/product/webhook"
	"storr-api/internal/store"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	cognito "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type authDeps struct {
	service *auth.Service
	handler *auth.Handler
}

type storeDeps struct {
	service *store.Service
	handler *store.Handler
}

type integrationDeps struct {
	handler         *integration.Handler
	callbackHandler *integration.CallbackHandler
}

type productDeps struct {
	handler        *product.Handler
	imageHandler   *image.Handler
	webhookHandler *webhook.Handler
}

type pipelineDeps struct {
	handler *pipeline.Handler
}

type allDeps struct {
	auth        authDeps
	store       storeDeps
	product     productDeps
	integration integrationDeps
	pipeline    pipelineDeps
}

func mustLoadAWSConfig(ctx context.Context) aws.Config {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		slog.Error("unable to load AWS config", "error", err)
		os.Exit(1)
	}
	return cfg
}

func newAuthDeps(cfg aws.Config, cog cognitoConfig) authDeps {
	client := cognito.NewFromConfig(cfg)
	svc := auth.NewService(client, cog.PoolID, cog.ClientID)
	return authDeps{service: svc, handler: auth.NewHandler(svc)}
}

func newStoreDeps(client *dynamodb.Client, table string) storeDeps {
	svc := store.NewService(client, table)
	return storeDeps{service: svc, handler: store.NewHandler(svc)}
}

func newProductDeps(repo *product.Repository, appCfg appConfig, secretsClient *secrets.SecretsClient, s3Client *s3.Client) productDeps {
	svc := product.NewService(repo, appCfg.Products.BatchUpdateMaxProducts)
	shopify := sync.NewShopifyClient()
	presignClient := s3.NewPresignClient(s3Client)
	imageSyncDeps := &sync.ImageSyncDeps{Presign: presignClient, S3: s3Client, Bucket: appCfg.Buckets.ProductImages}
	syncSvc := sync.NewService(repo, shopify, secretsClient, imageSyncDeps)
	imageSvc := image.NewService(repo, presignClient, s3Client, appCfg.Buckets.ProductImages)
	opts := product.HandlerOpts{
		GenerateXLSX: importexport.GenerateTemplate,
		GenerateCSV:  importexport.GenerateCSVTemplate,
		ParseCSV:     importexport.ParseCSV,
		ParseXLSX:    importexport.ParseXLSX,
	}
	return productDeps{
		handler:      product.NewHandler(svc, syncSvc, opts),
		imageHandler: image.NewHandler(imageSvc, syncSvc),
	}
}

func newWebhookHandler(repo *product.Repository, client *dynamodb.Client, appCfg appConfig, storeFinder webhook.StoreFinder) *webhook.Handler {
	eventStore := webhook.NewEventStore(client, appCfg.Tables.WebhookEvents)
	clientSecret := os.Getenv("SHOPIFY_CLIENT_SECRET")
	svc := webhook.NewService(repo, storeFinder)
	return webhook.NewHandler(svc, eventStore, clientSecret)
}

// sharedIntegrationServices holds integration services shared between the
// integration handler and the pipeline provisioner to avoid duplicate construction.
type sharedIntegrationServices struct {
	secrets    *secrets.SecretsClient
	stateStore *integration.OAuthStateStore
	figmaOAuth *integration.FigmaOAuthService
}

func newSharedIntegrationServices(cfg aws.Config, appCfg appConfig, storeSvc *store.Service) sharedIntegrationServices {
	smClient := secretsmanager.NewFromConfig(cfg)
	secretsClient := secrets.NewSecretsClient(smClient)
	dynamoClient := dynamodb.NewFromConfig(cfg)
	stateStore := integration.NewOAuthStateStore(dynamoClient, appCfg.Tables.OAuthStates)
	figmaOAuth := integration.NewFigmaOAuthService(loadFigmaOAuthConfig(), stateStore, secretsClient, storeSvc)
	return sharedIntegrationServices{
		secrets:    secretsClient,
		stateStore: stateStore,
		figmaOAuth: figmaOAuth,
	}
}

func newIntegrationDeps(shared sharedIntegrationServices, appCfg appConfig, storeSvc *store.Service) integrationDeps {
	shopifyOAuth := integration.NewShopifyOAuthService(loadShopifyOAuthConfig(), shared.stateStore, shared.secrets, storeSvc)
	handler := integration.NewHandler(shared.figmaOAuth, shopifyOAuth, storeSvc)
	callbackHandler := integration.NewCallbackHandler(shared.stateStore, shared.figmaOAuth, shopifyOAuth, appCfg.FrontendURL)
	return integrationDeps{handler: handler, callbackHandler: callbackHandler}
}

func newPipelineDeps(cfg aws.Config, ctx context.Context, storeSvc *store.Service, shared sharedIntegrationServices) pipelineDeps {
	previewRoute := newPreviewRouteManager(cfg)
	efsProvisioner := newEfsProvisioner(cfg)
	pipelineSvc := buildPipelineService(cfg, ctx, previewRoute)
	themeSvc := buildThemeService(cfg, ctx, previewRoute)
	provisioner := buildProvisioner(cfg, ctx, shared)
	services := pipeline.Services{
		Pipeline: pipelineSvc,
		Theme:    themeSvc,
		Stores:   storeSvc,
	}
	cleanup := pipeline.CleanupDeps{
		PreviewRoute: previewRoute,
		Efs:          efsProvisioner,
	}
	handler := pipeline.NewHandler(services, provisioner, cleanup)
	return pipelineDeps{handler: handler}
}

func newPreviewRouteManager(cfg aws.Config) *infra.PreviewRouteManager {
	clients := infra.AWSClients{
		EC2:   ec2.NewFromConfig(cfg),
		ELBV2: elasticloadbalancingv2.NewFromConfig(cfg),
	}
	return infra.NewPreviewRouteManager(clients, loadPreviewRouteConfig())
}

func newEfsProvisioner(cfg aws.Config) *infra.EfsProvisioner {
	fsID := os.Getenv("STUDIO_EFS_FILESYSTEM_ID")
	if fsID == "" {
		return nil
	}
	return infra.NewEfsProvisioner(efs.NewFromConfig(cfg), fsID)
}

func buildThemeService(cfg aws.Config, ctx context.Context, previewRoute *infra.PreviewRouteManager) *pipeline.ThemeService {
	themeCfg := loadThemeConfig()
	clients := infra.AWSClients{ECS: ecs.NewFromConfig(cfg), EC2: ec2.NewFromConfig(cfg)}
	runtimeToken := fetchRuntimeToken(cfg, ctx)
	rc := infra.NewRuntimeClient(runtimeToken)
	return pipeline.NewThemeService(clients, rc, previewRoute, themeCfg)
}

func buildPipelineService(cfg aws.Config, ctx context.Context, previewRoute *infra.PreviewRouteManager) *pipeline.Service {
	pipelineCfg := loadPipelineConfig()
	clients := infra.AWSClients{
		ECS:   ecs.NewFromConfig(cfg),
		EC2:   ec2.NewFromConfig(cfg),
		ELBV2: elasticloadbalancingv2.NewFromConfig(cfg),
	}
	runtimeToken := fetchRuntimeToken(cfg, ctx)
	rc := infra.NewRuntimeClient(runtimeToken)
	return pipeline.NewService(clients, rc, previewRoute, pipelineCfg)
}

func buildProvisioner(cfg aws.Config, ctx context.Context, shared sharedIntegrationServices) *integration.Provisioner {
	envParams := loadPipelineEnvParams()
	clients := integration.IntegrationClients{
		Secrets:    shared.secrets,
		FigmaOAuth: shared.figmaOAuth,
	}
	pipelineEnv := buildPipelineEnvConfig(cfg, ctx, envParams)
	return integration.NewProvisioner(clients, pipelineEnv)
}

func buildPipelineEnvConfig(cfg aws.Config, ctx context.Context, params pipelineEnvParams) integration.PipelineEnvConfig {
	anthropicKey := fetchSSMParam(cfg, ctx, params.AnthropicKeyParam)
	llmModel := fetchSSMParam(cfg, ctx, params.LlmModelParam)
	llmProvider := fetchSSMParam(cfg, ctx, params.LlmProviderParam)
	return integration.PipelineEnvConfig{
		AnthropicKey: anthropicKey,
		LlmModel:     llmModel,
		LlmProvider:  llmProvider,
	}
}

func fetchRuntimeToken(cfg aws.Config, ctx context.Context) string {
	paramName := os.Getenv("RUNTIME_AUTH_TOKEN_PARAM")
	return fetchSSMParam(cfg, ctx, paramName)
}

func fetchSSMParam(cfg aws.Config, ctx context.Context, paramName string) string {
	if paramName == "" {
		return ""
	}
	ssmClient := ssm.NewFromConfig(cfg)
	out, err := ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(paramName),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		slog.Warn("failed to read SSM param", "param", paramName, "error", err)
		return ""
	}
	return aws.ToString(out.Parameter.Value)
}
