package integration

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Service provides integration record persistence in DynamoDB.
type Service struct {
	client    *dynamodb.Client
	tableName string
}

// NewService creates an integration Service connected to the given DynamoDB table.
func NewService(client *dynamodb.Client, tableName string) *Service {
	return &Service{client: client, tableName: tableName}
}

// Put creates or updates an integration record.
func (s *Service) Put(ctx context.Context, integration Integration) error {
	item, err := attributevalue.MarshalMap(integration)
	if err != nil {
		return fmt.Errorf("marshal integration: %w", err)
	}

	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("put integration: %w", err)
	}

	return nil
}

// Get retrieves an integration record by customer and key.
func (s *Service) Get(ctx context.Context, customerID, integrationKey string) (*Integration, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"customerId":     &types.AttributeValueMemberS{Value: customerID},
			"integrationKey": &types.AttributeValueMemberS{Value: integrationKey},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get integration: %w", err)
	}
	if out.Item == nil {
		return nil, nil
	}

	var integration Integration
	if err := attributevalue.UnmarshalMap(out.Item, &integration); err != nil {
		return nil, fmt.Errorf("unmarshal integration: %w", err)
	}

	return &integration, nil
}

// List returns all integration records for a customer.
func (s *Service) List(ctx context.Context, customerID string) ([]Integration, error) {
	out, err := s.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.tableName),
		KeyConditionExpression: aws.String("customerId = :cid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":cid": &types.AttributeValueMemberS{Value: customerID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("query integrations: %w", err)
	}

	integrations := make([]Integration, 0, len(out.Items))
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &integrations); err != nil {
		return nil, fmt.Errorf("unmarshal integrations: %w", err)
	}

	return integrations, nil
}
