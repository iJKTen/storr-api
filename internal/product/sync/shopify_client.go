package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"storr-api/internal/httpclient"
	"storr-api/internal/product"
	"storr-api/internal/secrets"
)

// ShopifyClient provides methods for product operations via the Shopify Admin GraphQL API.
type ShopifyClient struct{}

// NewShopifyClient creates a new ShopifyClient.
func NewShopifyClient() *ShopifyClient {
	return &ShopifyClient{}
}

// --- Fetch Products ---

// FetchProducts retrieves a page of products from Shopify and converts them to Storr format.
func (c *ShopifyClient) FetchProducts(ctx context.Context, creds ShopifyCreds, cursor string) (*ShopifyProductPage, error) {
	body, err := buildFetchBody(cursor)
	if err != nil {
		return nil, err
	}
	resp, err := decodeShopifyGQL[productsGQLResponse](ctx, creds, body)
	if err != nil {
		return nil, err
	}
	return extractProductPage(*resp)
}

func buildFetchBody(cursor string) ([]byte, error) {
	vars := fetchProductsVars{First: shopifyPageSize}
	if cursor != "" {
		vars.After = cursor
	}
	return marshalGQLBody(fetchProductsQuery, vars)
}

func extractProductPage(resp productsGQLResponse) (*ShopifyProductPage, error) {
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("shopify products query: %s", resp.Errors[0].Message)
	}
	conn := resp.Data.Products
	products := make([]product.Product, len(conn.Edges))
	for i, e := range conn.Edges {
		products[i] = shopifyProductToStorr(e.Node)
	}
	return &ShopifyProductPage{
		Products: products,
		Cursor:   conn.PageInfo.EndCursor,
		HasNext:  conn.PageInfo.HasNextPage,
	}, nil
}

// --- Fetch Single Product ---

// FetchProduct retrieves a single product from Shopify by its GID and converts it to Storr format.
func (c *ShopifyClient) FetchProduct(ctx context.Context, creds ShopifyCreds, shopifyID string) (*product.Product, error) {
	body, err := buildFetchByIDBody(shopifyID)
	if err != nil {
		return nil, err
	}
	resp, err := decodeShopifyGQL[nodeGQLResponse](ctx, creds, body)
	if err != nil {
		return nil, err
	}
	return extractSingleProduct(*resp)
}

func buildFetchByIDBody(shopifyID string) ([]byte, error) {
	vars := fetchProductByIDVars{ID: shopifyID}
	return marshalGQLBody(fetchProductByIDQuery, vars)
}

func extractSingleProduct(resp nodeGQLResponse) (*product.Product, error) {
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("shopify product query: %s", resp.Errors[0].Message)
	}
	if resp.Data.Node.ID == "" {
		return nil, fmt.Errorf("%w: shopify product not found", product.ErrProductNotFound)
	}
	p := shopifyProductToStorr(resp.Data.Node)
	return &p, nil
}

// --- Upsert Product ---

// UpsertProduct creates or updates a product in Shopify using the productSet mutation.
// Returns the Shopify product GID on success.
func (c *ShopifyClient) UpsertProduct(ctx context.Context, creds ShopifyCreds, input ProductSetInput) (string, error) {
	body, err := buildUpsertBody(input)
	if err != nil {
		return "", err
	}
	resp, err := decodeShopifyGQL[productSetGQLResponse](ctx, creds, body)
	if err != nil {
		return "", err
	}
	return extractUpsertedID(*resp)
}

func buildUpsertBody(input ProductSetInput) ([]byte, error) {
	vars := upsertProductVars{Input: input, Synchronous: true}
	return marshalGQLBody(upsertProductMutation, vars)
}

func extractUpsertedID(resp productSetGQLResponse) (string, error) {
	if len(resp.Errors) > 0 {
		return "", fmt.Errorf("shopify productSet: %s", resp.Errors[0].Message)
	}
	result := resp.Data.ProductSet
	if len(result.UserErrors) > 0 {
		return "", fmt.Errorf("shopify productSet: %s", result.UserErrors[0].Message)
	}
	if result.Product == nil {
		return "", fmt.Errorf("shopify productSet returned no product")
	}
	return result.Product.ID, nil
}

// --- GraphQL Helpers ---

func sendShopifyGQL(ctx context.Context, creds ShopifyCreds, body []byte) (*http.Response, error) {
	url := shopifyGQLURL(creds.Domain)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Access-Token", creds.AdminToken)
	resp, err := httpclient.Default.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("shopify api: status %d", resp.StatusCode)
	}
	return resp, nil
}

func decodeShopifyGQL[T productsGQLResponse | productSetGQLResponse | nodeGQLResponse](ctx context.Context, creds ShopifyCreds, body []byte) (*T, error) {
	resp, err := sendShopifyGQL(ctx, creds, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode shopify response: %w", err)
	}
	return &result, nil
}

func marshalGQLBody[V fetchProductsVars | upsertProductVars | fetchProductByIDVars](query string, vars V) ([]byte, error) {
	type body struct {
		Query     string `json:"query"`
		Variables V      `json:"variables,omitempty"`
	}
	return json.Marshal(body{Query: query, Variables: vars})
}

func shopifyGQLURL(domain string) string {
	return fmt.Sprintf("%s/admin/api/%s/graphql.json", domain, secrets.ShopifyAPIVersion)
}

// --- GraphQL Queries ---

const fetchProductsQuery = `query($first: Int!, $after: String) {
  products(first: $first, after: $after) {
    edges {
      node {
        id handle title descriptionHtml vendor productType tags status
        publishedAt templateSuffix isGiftCard
        seo { title description }
        metafields(first: 50) {
          nodes { namespace key value type }
        }
        options { name position values }
        variants(first: 100) {
          edges {
            node {
              id title sku barcode price compareAtPrice position
              inventoryQuantity inventoryPolicy
              taxable taxCode requiresShipping weight weightUnit
              inventoryItem {
                unitCost { amount }
              }
              selectedOptions { name value }
              image { id }
            }
          }
        }
        images(first: 250) {
          edges {
            node { id url altText width height }
          }
        }
      }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

const fetchProductByIDQuery = `query($id: ID!) {
  node(id: $id) {
    ... on Product {
      id handle title descriptionHtml vendor productType tags status
      publishedAt templateSuffix isGiftCard
      seo { title description }
      metafields(first: 50) {
        nodes { namespace key value type }
      }
      options { name position values }
      variants(first: 100) {
        edges {
          node {
            id title sku barcode price compareAtPrice position
            inventoryQuantity inventoryPolicy
            taxable taxCode requiresShipping weight weightUnit
            inventoryItem {
              unitCost { amount }
            }
            selectedOptions { name value }
            image { id }
          }
        }
      }
      images(first: 250) {
        edges {
          node { id url altText width height }
        }
      }
    }
  }
}`

const upsertProductMutation = `mutation productSet($input: ProductSetInput!, $synchronous: Boolean!) {
  productSet(input: $input, synchronous: $synchronous) {
    product { id }
    userErrors { field message }
  }
}`

// --- Webhook Registration ---

// RegisterProductWebhooks creates webhook subscriptions for product events on the connected Shopify store.
func RegisterProductWebhooks(ctx context.Context, creds ShopifyCreds, callbackURL string) error {
	for _, topic := range productWebhookTopics {
		if err := registerSingleWebhook(ctx, creds, topic, callbackURL); err != nil {
			return fmt.Errorf("register webhook %s: %w", topic, err)
		}
	}
	return nil
}

var productWebhookTopics = []string{
	"PRODUCTS_CREATE",
	"PRODUCTS_UPDATE",
	"PRODUCTS_DELETE",
}

func registerSingleWebhook(ctx context.Context, creds ShopifyCreds, topic, callbackURL string) error {
	body, err := buildWebhookRegBody(topic, callbackURL)
	if err != nil {
		return err
	}
	resp, err := sendShopifyGQL(ctx, creds, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeWebhookRegResponse(resp.Body)
}

func buildWebhookRegBody(topic, callbackURL string) ([]byte, error) {
	type vars struct {
		Topic       string `json:"topic"`
		CallbackURL string `json:"callbackUrl"`
	}
	type body struct {
		Query     string `json:"query"`
		Variables vars   `json:"variables"`
	}
	return json.Marshal(body{
		Query:     webhookCreateMutation,
		Variables: vars{Topic: topic, CallbackURL: callbackURL},
	})
}

func decodeWebhookRegResponse(r io.Reader) error {
	var result webhookCreateResponse
	if err := json.NewDecoder(r).Decode(&result); err != nil {
		return fmt.Errorf("decode webhook response: %w", err)
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("webhook create: %s", result.Errors[0].Message)
	}
	ue := result.Data.WebhookSubscriptionCreate.UserErrors
	if len(ue) > 0 {
		return fmt.Errorf("webhook create: %s", ue[0].Message)
	}
	return nil
}

const webhookCreateMutation = `mutation webhookSubscriptionCreate($topic: WebhookSubscriptionTopic!, $callbackUrl: URL!) {
  webhookSubscriptionCreate(topic: $topic, webhookSubscription: {callbackUrl: $callbackUrl, format: JSON}) {
    webhookSubscription { id }
    userErrors { field message }
  }
}`

type webhookCreateResponse struct {
	Data   webhookCreateData `json:"data"`
	Errors []shopifyGQLError `json:"errors"`
}

type webhookCreateData struct {
	WebhookSubscriptionCreate webhookCreateResult `json:"webhookSubscriptionCreate"`
}

type webhookCreateResult struct {
	UserErrors []shopifyUError `json:"userErrors"`
}
