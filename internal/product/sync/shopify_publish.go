package sync

import (
	"context"
	"encoding/json"
	"fmt"
)

const (
	onlineStoreCatalogTitle = "Online Store"
	publicationsQueryLimit  = 20
)

const publicationsQuery = `query($first: Int!) {
  publications(first: $first) {
    nodes { id catalog { title } }
  }
}`

const publishMutation = `mutation publishablePublish($id: ID!, $input: [PublicationInput!]!) {
  publishablePublish(id: $id, input: $input) {
    userErrors { field message }
  }
}`

const unpublishMutation = `mutation publishableUnpublish($id: ID!, $input: [PublicationInput!]!) {
  publishableUnpublish(id: $id, input: $input) {
    userErrors { field message }
  }
}`

// FetchOnlineStorePublicationID queries Shopify for the Online Store
// publication GID, returning an empty string if not found.
func (c *ShopifyClient) FetchOnlineStorePublicationID(ctx context.Context, creds ShopifyCreds) (string, error) {
	body, err := buildPublicationsQueryBody()
	if err != nil {
		return "", fmt.Errorf("build publications query: %w", err)
	}
	resp, err := decodePublishGQL[publicationsGQLResponse](ctx, creds, body)
	if err != nil {
		return "", fmt.Errorf("fetch publications: %w", err)
	}
	return extractOnlineStorePubID(*resp), nil
}

// PublishProduct makes a product visible on the given sales channel.
func (c *ShopifyClient) PublishProduct(ctx context.Context, creds ShopifyCreds, productID, publicationID string) error {
	body, err := buildPublishBody(productID, publicationID)
	if err != nil {
		return fmt.Errorf("build publish body: %w", err)
	}
	resp, err := decodePublishGQL[publishGQLResponse](ctx, creds, body)
	if err != nil {
		return fmt.Errorf("publish product: %w", err)
	}
	return checkPublishErrors(*resp)
}

// UnpublishProduct removes a product from the given sales channel.
func (c *ShopifyClient) UnpublishProduct(ctx context.Context, creds ShopifyCreds, productID, publicationID string) error {
	body, err := buildUnpublishBody(productID, publicationID)
	if err != nil {
		return fmt.Errorf("build unpublish body: %w", err)
	}
	resp, err := decodePublishGQL[publishGQLResponse](ctx, creds, body)
	if err != nil {
		return fmt.Errorf("unpublish product: %w", err)
	}
	return checkPublishErrors(*resp)
}

func buildPublicationsQueryBody() ([]byte, error) {
	vars := publicationsQueryVars{First: publicationsQueryLimit}
	return marshalPublishGQLBody(publicationsQuery, vars)
}

func extractOnlineStorePubID(resp publicationsGQLResponse) string {
	for _, node := range resp.Data.Publications.Nodes {
		if node.Catalog.Title == onlineStoreCatalogTitle {
			return node.ID
		}
	}
	return ""
}

func buildPublishBody(productID, publicationID string) ([]byte, error) {
	vars := publicationVars{
		ID:    productID,
		Input: []publicationInput{{PublicationID: publicationID}},
	}
	return marshalPublishGQLBody(publishMutation, vars)
}

func buildUnpublishBody(productID, publicationID string) ([]byte, error) {
	vars := publicationVars{
		ID:    productID,
		Input: []publicationInput{{PublicationID: publicationID}},
	}
	return marshalPublishGQLBody(unpublishMutation, vars)
}

func checkPublishErrors(resp publishGQLResponse) error {
	if len(resp.Errors) > 0 {
		return fmt.Errorf("shopify publish: %s", resp.Errors[0].Message)
	}
	result := resp.Data.Publish
	if result == nil {
		result = resp.Data.Unpublish
	}
	if result != nil && len(result.UserErrors) > 0 {
		return fmt.Errorf("shopify publish: %s", result.UserErrors[0].Message)
	}
	return nil
}

func decodePublishGQL[T publicationsGQLResponse | publishGQLResponse](ctx context.Context, creds ShopifyCreds, body []byte) (*T, error) {
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

func marshalPublishGQLBody[V publicationsQueryVars | publicationVars](query string, vars V) ([]byte, error) {
	type body struct {
		Query     string `json:"query"`
		Variables V      `json:"variables,omitempty"`
	}
	return json.Marshal(body{Query: query, Variables: vars})
}
