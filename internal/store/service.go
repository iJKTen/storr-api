package store

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// FigmaFileKeyPattern matches a Figma file key from a design or file URL.
const FigmaFileKeyPattern = `figma\.com/(?:design|file)/([a-zA-Z0-9]+)`

var (
	// ErrStoreNotFound is returned when a store lookup finds no matching item.
	ErrStoreNotFound    = errors.New("store not found")
	figmaFileKeyRegex   = regexp.MustCompile(FigmaFileKeyPattern)
)

// Service provides store persistence operations against DynamoDB.
type Service struct {
	client    *dynamodb.Client
	tableName string
}

// NewService creates a store service connected to the given DynamoDB table.
func NewService(client *dynamodb.Client, tableName string) *Service {
	return &Service{client: client, tableName: tableName}
}

// List returns all stores owned by the specified user.
func (s *Service) List(ctx context.Context, userID string) ([]Store, error) {
	out, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		KeyConditionExpression: aws.String("userId = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("query stores: %w", err)
	}

	stores := make([]Store, 0, len(out.Items))
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &stores); err != nil {
		return nil, fmt.Errorf("unmarshal stores: %w", err)
	}

	return stores, nil
}

// Get retrieves a single store by user and store ID.
func (s *Service) Get(ctx context.Context, userID, storeID string) (*Store, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key:       storeKey(userID, storeID),
	})
	if err != nil {
		return nil, fmt.Errorf("get store: %w", err)
	}
	if out.Item == nil {
		return nil, ErrStoreNotFound
	}

	var store Store
	if err := attributevalue.UnmarshalMap(out.Item, &store); err != nil {
		return nil, fmt.Errorf("unmarshal store: %w", err)
	}

	return &store, nil
}

// Create persists a new store derived from the given request.
func (s *Service) Create(ctx context.Context, userID string, req CreateStoreRequest) (*Store, error) {
	store := newStoreFromRequest(userID, req)
	item, err := attributevalue.MarshalMap(store)
	if err != nil {
		return nil, fmt.Errorf("marshal store: %w", err)
	}
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	})
	if err != nil {
		return nil, fmt.Errorf("put store: %w", err)
	}
	return &store, nil
}

// Update applies changes to an existing store and persists the result.
func (s *Service) Update(ctx context.Context, userID, storeID string, req UpdateStoreRequest) (*Store, error) {
	existing, err := s.Get(ctx, userID, storeID)
	if err != nil {
		return nil, err
	}
	applyUpdate(existing, req)
	item, err := attributevalue.MarshalMap(*existing)
	if err != nil {
		return nil, fmt.Errorf("marshal store: %w", err)
	}
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	})
	if err != nil {
		return nil, fmt.Errorf("put store: %w", err)
	}
	return existing, nil
}

// GetIntegrations returns the integration configs for a store.
func (s *Service) GetIntegrations(ctx context.Context, userID, storeID string) (map[string]IntegrationConfig, error) {
	store, err := s.Get(ctx, userID, storeID)
	if err != nil {
		return nil, err
	}
	if store.Integrations == nil {
		return map[string]IntegrationConfig{}, nil
	}
	return store.Integrations, nil
}

// PutIntegration atomically upserts a single integration config on a store.
// Uses if_not_exists to initialize the integrations map if absent, then sets
// the specific key — all in one UpdateItem call. This prevents lost updates
// when multiple integrations are written concurrently.
func (s *Service) PutIntegration(ctx context.Context, userID, storeID string, update IntegrationUpdate) error {
	av, err := attributevalue.Marshal(update.Config)
	if err != nil {
		return fmt.Errorf("marshal integration config: %w", err)
	}
	_, err = s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.tableName),
		Key:       storeKey(userID, storeID),
		UpdateExpression: aws.String(
			"SET #integrations = if_not_exists(#integrations, :empty), #integrations.#key = :val",
		),
		ExpressionAttributeNames: map[string]string{
			"#integrations": "integrations",
			"#key":          update.Key,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":val":   av,
			":empty": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
		},
		ConditionExpression: aws.String("attribute_exists(userId)"),
	})
	if err != nil {
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			return ErrStoreNotFound
		}
		return fmt.Errorf("put integration: %w", err)
	}
	return nil
}

// DeleteIntegration removes a single integration key from a store.
func (s *Service) DeleteIntegration(ctx context.Context, userID, storeID, key string) error {
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        aws.String(s.tableName),
		Key:              storeKey(userID, storeID),
		UpdateExpression: aws.String("REMOVE integrations.#key"),
		ExpressionAttributeNames: map[string]string{
			"#key": key,
		},
		ConditionExpression: aws.String("attribute_exists(userId)"),
	})
	if err != nil {
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			return ErrStoreNotFound
		}
		return fmt.Errorf("delete integration: %w", err)
	}
	return nil
}

// Delete removes a store from DynamoDB.
func (s *Service) Delete(ctx context.Context, userID, storeID string) error {
	_, err := s.Get(ctx, userID, storeID)
	if err != nil {
		return err
	}

	_, err = s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.tableName),
		Key:       storeKey(userID, storeID),
	})
	if err != nil {
		return fmt.Errorf("delete store: %w", err)
	}

	return nil
}

// FindByShopifyURL queries the shopifyUrl-index GSI for a store matching the
// given Shopify URL and returns its owner and store IDs.
//
// Requires a GSI named "shopifyUrl-index" with partition key "shopifyUrl"
// to be provisioned on the stores table in CloudFormation.
func (s *Service) FindByShopifyURL(ctx context.Context, shopifyURL string) (string, string, error) {
	out, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		IndexName:              aws.String(ShopifyURLIndexName),
		KeyConditionExpression: aws.String("shopifyUrl = :url"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":url": &types.AttributeValueMemberS{Value: shopifyURL},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return "", "", fmt.Errorf("query shopify url index: %w", err)
	}
	if len(out.Items) == 0 {
		return "", "", ErrStoreNotFound
	}
	var store Store
	if err := attributevalue.UnmarshalMap(out.Items[0], &store); err != nil {
		return "", "", fmt.Errorf("unmarshal store: %w", err)
	}
	return store.UserID, store.ID, nil
}

func newStoreFromRequest(userID string, req CreateStoreRequest) Store {
	return Store{
		ID:            StoreIDPrefix + uuid.NewString(),
		UserID:        userID,
		Name:          req.Name,
		ProjectType:   resolveProjectType(req.ProjectType),
		ShopifyURL:    req.ShopifyURL,
		FigmaFileKeys: extractFileKeys(req.FigmaURLs),
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
}

func applyUpdate(existing *Store, req UpdateStoreRequest) {
	existing.Name = req.Name
	if req.ShopifyURL != "" {
		existing.ShopifyURL = req.ShopifyURL
	}
	if len(req.FigmaURLs) > 0 {
		existing.FigmaFileKeys = extractFileKeys(req.FigmaURLs)
	}
}

func storeKey(userID, storeID string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"userId":  &types.AttributeValueMemberS{Value: userID},
		"storeId": &types.AttributeValueMemberS{Value: storeID},
	}
}

func resolveProjectType(raw string) ProjectType {
	if ProjectType(raw) == ProjectTypeTheme {
		return ProjectTypeTheme
	}
	return ProjectTypeHydrogen
}

func extractFileKeys(urls []string) []string {
	keys := make([]string, 0, len(urls))
	for _, u := range urls {
		matches := figmaFileKeyRegex.FindStringSubmatch(u)
		if len(matches) >= 2 {
			keys = append(keys, matches[1])
		}
	}
	return keys
}
