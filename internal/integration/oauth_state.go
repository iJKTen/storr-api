package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// OAuthState is a DynamoDB record tracking an in-progress OAuth authorization.
type OAuthState struct {
	State      string `dynamodbav:"state"`
	CustomerID string `dynamodbav:"customerId"`
	Provider   string `dynamodbav:"provider"`
	StoreID    string `dynamodbav:"storeId,omitempty"`
	Shop       string `dynamodbav:"shop,omitempty"`
	TTL        int64  `dynamodbav:"ttl"`
}

// OAuthStateStore persists CSRF state tokens in DynamoDB with a TTL.
type OAuthStateStore struct {
	client    *dynamodb.Client
	tableName string
}

// NewOAuthStateStore creates an OAuthStateStore connected to the given DynamoDB table.
func NewOAuthStateStore(client *dynamodb.Client, tableName string) *OAuthStateStore {
	return &OAuthStateStore{client: client, tableName: tableName}
}

// Put stores an OAuth state record with a TTL for automatic expiry.
func (s *OAuthStateStore) Put(ctx context.Context, state OAuthState) error {
	state.TTL = time.Now().Add(OAuthStateTTL).Unix()
	item, err := attributevalue.MarshalMap(state)
	if err != nil {
		return fmt.Errorf("marshal oauth state: %w", err)
	}
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("put oauth state: %w", err)
	}
	return nil
}

// Get retrieves an OAuth state record by its state token.
func (s *OAuthStateStore) Get(ctx context.Context, state string) (*OAuthState, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"state": &types.AttributeValueMemberS{Value: state},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get oauth state: %w", err)
	}
	if out.Item == nil {
		return nil, nil
	}
	var result OAuthState
	if err := attributevalue.UnmarshalMap(out.Item, &result); err != nil {
		return nil, fmt.Errorf("unmarshal oauth state: %w", err)
	}
	return &result, nil
}

// Delete removes an OAuth state record after it has been consumed.
func (s *OAuthStateStore) Delete(ctx context.Context, state string) error {
	_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.tableName),
		Key: map[string]types.AttributeValue{
			"state": &types.AttributeValueMemberS{Value: state},
		},
	})
	if err != nil {
		return fmt.Errorf("delete oauth state: %w", err)
	}
	return nil
}
