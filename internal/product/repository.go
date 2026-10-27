package product

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	batchWriteMaxRetries = 3
	batchWriteBaseDelay  = 100 * time.Millisecond
)

// Repository provides product persistence operations against DynamoDB.
type Repository struct {
	client    *dynamodb.Client
	tableName string
}

// NewRepository creates a product repository connected to the given DynamoDB table.
func NewRepository(client *dynamodb.Client, tableName string) *Repository {
	return &Repository{client: client, tableName: tableName}
}

// Create persists a new product to DynamoDB.
func (r *Repository) Create(ctx context.Context, p Product) error {
	item, err := attributevalue.MarshalMap(p)
	if err != nil {
		return fmt.Errorf("marshal product: %w", err)
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("put product: %w", err)
	}
	return nil
}

// Get retrieves a single product by store and product ID.
func (r *Repository) Get(ctx context.Context, storeID, productID string) (*Product, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key:       productKey(storeID, productID),
	})
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	if out.Item == nil {
		return nil, ErrProductNotFound
	}
	return unmarshalProduct(out.Item)
}

// List returns a page of products for a store using cursor-based pagination.
// Direction controls sort order: PageForward (default) scans ascending,
// PageBackward scans descending and reverses results before returning.
func (r *Repository) List(ctx context.Context, storeID, cursor string, limit int, direction PageDirection) (*ListResult, error) {
	pageSize := clampPageSize(limit)
	input := r.buildListInput(storeID, cursor, pageSize, direction)
	out, err := r.client.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("query products: %w", err)
	}
	products, err := unmarshalProducts(out.Items)
	if err != nil {
		return nil, err
	}

	result := &ListResult{}
	if len(products) == 0 {
		result.Products = products
		return result, nil
	}

	lastKey := encodeLastKey(out.LastEvaluatedKey)

	switch direction {
	case PageBackward:
		reverseProducts(products)
		result.Products = products
		result.NextCursor = products[len(products)-1].ProductID
		result.PrevCursor = lastKey
	default: // PageForward
		result.Products = products
		result.NextCursor = lastKey
		if cursor != "" {
			result.PrevCursor = products[0].ProductID
		}
	}

	return result, nil
}

// Update overwrites an existing product in DynamoDB.
func (r *Repository) Update(ctx context.Context, p Product) error {
	item, err := attributevalue.MarshalMap(p)
	if err != nil {
		return fmt.Errorf("marshal product: %w", err)
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("put product: %w", err)
	}
	return nil
}

// Delete removes a product from DynamoDB.
func (r *Repository) Delete(ctx context.Context, storeID, productID string) error {
	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key:       productKey(storeID, productID),
	})
	if err != nil {
		return fmt.Errorf("delete product: %w", err)
	}
	return nil
}

// GetByShopifyID looks up a product by its Shopify ID using the GSI.
func (r *Repository) GetByShopifyID(ctx context.Context, storeID, shopifyID string) (*Product, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String("shopifyId-index"),
		KeyConditionExpression: aws.String("storeId = :sid AND shopifyId = :shid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":sid":  &types.AttributeValueMemberS{Value: storeID},
			":shid": &types.AttributeValueMemberS{Value: shopifyID},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("query by shopify id: %w", err)
	}
	if len(out.Items) == 0 {
		return nil, ErrProductNotFound
	}
	return unmarshalProduct(out.Items[0])
}

// BatchWrite persists multiple products in DynamoDB BatchWriteItem chunks.
func (r *Repository) BatchWrite(ctx context.Context, products []Product) error {
	requests, err := buildWriteRequests(products)
	if err != nil {
		return err
	}
	for i := 0; i < len(requests); i += BatchWriteChunkSize {
		chunk := requests[i:min(i+BatchWriteChunkSize, len(requests))]
		if err := r.writeBatch(ctx, chunk); err != nil {
			return err
		}
	}
	return nil
}

// BatchGet retrieves multiple products by their IDs in a single DynamoDB BatchGetItem call.
// Returns a map keyed by productID for O(1) lookup. Handles the 100-key-per-request
// DynamoDB limit and retries unprocessed keys with exponential backoff.
func (r *Repository) BatchGet(ctx context.Context, storeID string, productIDs []string) (map[string]Product, error) {
	result := make(map[string]Product, len(productIDs))
	for i := 0; i < len(productIDs); i += BatchGetChunkSize {
		chunk := productIDs[i:min(i+BatchGetChunkSize, len(productIDs))]
		keys := make([]map[string]types.AttributeValue, len(chunk))
		for j, pid := range chunk {
			keys[j] = productKey(storeID, pid)
		}
		if err := r.batchGet(ctx, keys, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *Repository) batchGet(ctx context.Context, keys []map[string]types.AttributeValue, result map[string]Product) error {
	unprocessed := map[string]types.KeysAndAttributes{
		r.tableName: {Keys: keys},
	}
	for attempt := 0; attempt < batchWriteMaxRetries; attempt++ {
		out, err := r.client.BatchGetItem(ctx, &dynamodb.BatchGetItemInput{
			RequestItems: unprocessed,
		})
		if err != nil {
			return fmt.Errorf("batch get products: %w", err)
		}
		for _, item := range out.Responses[r.tableName] {
			p, err := unmarshalProduct(item)
			if err != nil {
				return err
			}
			result[p.ProductID] = *p
		}
		if len(out.UnprocessedKeys) == 0 {
			return nil
		}
		unprocessed = out.UnprocessedKeys
		sleepDuration := batchWriteBaseDelay << attempt
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleepDuration):
		}
	}
	return fmt.Errorf("batch get: %d unprocessed keys after retries", len(unprocessed[r.tableName].Keys))
}

func (r *Repository) writeBatch(ctx context.Context, requests []types.WriteRequest) error {
	unprocessed := map[string][]types.WriteRequest{r.tableName: requests}
	for attempt := 0; attempt < batchWriteMaxRetries; attempt++ {
		out, err := r.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: unprocessed,
		})
		if err != nil {
			return fmt.Errorf("batch write products: %w", err)
		}
		if len(out.UnprocessedItems) == 0 {
			return nil
		}
		unprocessed = out.UnprocessedItems
		sleepDuration := batchWriteBaseDelay << attempt
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleepDuration):
		}
	}
	return fmt.Errorf("batch write: %d unprocessed items after retries", len(unprocessed[r.tableName]))
}

func (r *Repository) buildListInput(storeID, cursor string, limit int, direction PageDirection) *dynamodb.QueryInput {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("storeId = :sid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":sid": &types.AttributeValueMemberS{Value: storeID},
		},
		Limit: aws.Int32(int32(limit)),
	}
	if direction == PageBackward {
		input.ScanIndexForward = aws.Bool(false)
	}
	if cursor != "" {
		input.ExclusiveStartKey = decodeCursor(storeID, cursor)
	}
	return input
}

func reverseProducts(products []Product) {
	for i, j := 0, len(products)-1; i < j; i, j = i+1, j-1 {
		products[i], products[j] = products[j], products[i]
	}
}

func productKey(storeID, productID string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"storeId":   &types.AttributeValueMemberS{Value: storeID},
		"productId": &types.AttributeValueMemberS{Value: productID},
	}
}

func unmarshalProduct(item map[string]types.AttributeValue) (*Product, error) {
	var p Product
	if err := attributevalue.UnmarshalMap(item, &p); err != nil {
		return nil, fmt.Errorf("unmarshal product: %w", err)
	}
	return &p, nil
}

func unmarshalProducts(items []map[string]types.AttributeValue) ([]Product, error) {
	products := make([]Product, 0, len(items))
	if err := attributevalue.UnmarshalListOfMaps(items, &products); err != nil {
		return nil, fmt.Errorf("unmarshal products: %w", err)
	}
	return products, nil
}

func buildWriteRequests(products []Product) ([]types.WriteRequest, error) {
	requests := make([]types.WriteRequest, 0, len(products))
	for _, p := range products {
		item, err := attributevalue.MarshalMap(p)
		if err != nil {
			return nil, fmt.Errorf("marshal product for batch: %w", err)
		}
		requests = append(requests, types.WriteRequest{
			PutRequest: &types.PutRequest{Item: item},
		})
	}
	return requests, nil
}

func clampPageSize(limit int) int {
	if limit <= 0 || limit > MaxPageSize {
		return DefaultPageSize
	}
	return limit
}

func encodeLastKey(key map[string]types.AttributeValue) string {
	if key == nil {
		return ""
	}
	val, ok := key["productId"].(*types.AttributeValueMemberS)
	if !ok {
		return ""
	}
	return val.Value
}

func decodeCursor(storeID, cursor string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"storeId":   &types.AttributeValueMemberS{Value: storeID},
		"productId": &types.AttributeValueMemberS{Value: cursor},
	}
}
