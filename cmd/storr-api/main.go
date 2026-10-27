package main

import (
	"context"

	"storr-api/internal/logging"
	"storr-api/internal/product"
	"storr-api/internal/secrets"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
)

var chiLambda *chiadapter.ChiLambda

func init() {
	logging.Init()
	ctx := context.Background()
	appCfg := loadConfig()
	if err := appCfg.validate(); err != nil {
		panic(err)
	}
	awsCfg := mustLoadAWSConfig(ctx)
	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	deps := buildAllDeps(ctx, awsCfg, dynamoClient, appCfg)
	chiLambda = chiadapter.New(newRouter(deps, appCfg.CORS.Origin))
}

func buildAllDeps(ctx context.Context, awsCfg aws.Config, dynamoClient *dynamodb.Client, appCfg appConfig) allDeps {
	authD := newAuthDeps(awsCfg, loadCognitoConfig())
	storeD := newStoreDeps(dynamoClient, appCfg.Tables.Stores)
	shared := newSharedIntegrationServices(awsCfg, appCfg, storeD.service)
	s3Client := s3.NewFromConfig(awsCfg)
	repo := product.NewRepository(dynamoClient, appCfg.Tables.Products)
	productD := newProductDeps(repo, appCfg, shared.secrets, s3Client)
	productD.webhookHandler = newWebhookHandler(repo, dynamoClient, appCfg, storeD.service)
	integrationD := newIntegrationDeps(shared, appCfg, storeD.service)
	pipelineD := newPipelineDeps(awsCfg, ctx, storeD.service, shared)
	registerStoreCleanup(storeD, pipelineD, shared)
	return allDeps{
		auth:        authD,
		store:       storeD,
		product:     productD,
		integration: integrationD,
		pipeline:    pipelineD,
	}
}

func registerStoreCleanup(storeD storeDeps, pipelineD pipelineDeps, shared sharedIntegrationServices) {
	storeD.handler.SetCleanup(func(ctx context.Context, userID, storeID string) error {
		pipelineD.handler.CleanupStore(ctx, storeID)
		secrets.DeleteStoreSecrets(shared.secrets, ctx, userID, storeID)
		return nil
	})
}

// Handler is the AWS Lambda entry point that proxies API Gateway requests through the Chi router.
func Handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	return chiLambda.ProxyWithContext(ctx, req)
}

func main() {
	lambda.Start(Handler)
}
