package webhook

import (
	"context"
	"errors"
	"fmt"
	"time"

	"storr-api/internal/product"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// EventStore provides deduplication for incoming Shopify webhooks.
type EventStore struct {
	client    *dynamodb.Client
	tableName string
}

// NewEventStore creates a webhook event store backed by the given DynamoDB table.
func NewEventStore(client *dynamodb.Client, tableName string) *EventStore {
	return &EventStore{client: client, tableName: tableName}
}

type webhookEvent struct {
	EventID   string `dynamodbav:"eventId"`
	ExpiresAt int64  `dynamodbav:"expiresAt"`
}

// TryRecord atomically claims a webhook event ID. Returns true if the event was
// newly recorded (caller should process it), or false if it already existed
// (duplicate — caller should skip). This uses a conditional PutItem with
// attribute_not_exists to eliminate the TOCTOU race in the previous
// GetItem → process → PutItem pattern.
func (s *EventStore) TryRecord(ctx context.Context, eventID string) (bool, error) {
	ttl := time.Now().Add(product.WebhookEventTTLHours * time.Hour).Unix()
	item, err := attributevalue.MarshalMap(webhookEvent{EventID: eventID, ExpiresAt: ttl})
	if err != nil {
		return false, fmt.Errorf("marshal webhook event: %w", err)
	}
	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(s.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(eventId)"),
	})
	if err != nil {
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			return false, nil
		}
		return false, fmt.Errorf("record webhook event: %w", err)
	}
	return true, nil
}
