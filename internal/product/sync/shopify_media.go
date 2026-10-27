package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"storr-api/internal/httpclient"
)

// --- CreateStagedUpload ---

// CreateStagedUpload requests a staging URL from Shopify for a single file upload.
func (c *ShopifyClient) CreateStagedUpload(ctx context.Context, creds ShopifyCreds, fileName, mimeType string) (*StagedUploadTarget, error) {
	body, err := buildStagedUploadBody(fileName, mimeType)
	if err != nil {
		return nil, err
	}
	resp, err := decodeMediaGQL[stagedUploadGQLResponse](ctx, creds, body)
	if err != nil {
		return nil, err
	}
	return extractStagedTarget(*resp)
}

func buildStagedUploadBody(fileName, mimeType string) ([]byte, error) {
	vars := stagedUploadVars{
		Input: []stagedUploadInput{{
			Resource:   "IMAGE",
			Filename:   fileName,
			MimeType:   mimeType,
			HttpMethod: "POST",
		}},
	}
	return marshalMediaGQLBody(stagedUploadsCreateMutation, vars)
}

func extractStagedTarget(resp stagedUploadGQLResponse) (*StagedUploadTarget, error) {
	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("shopify stagedUpload: %s", resp.Errors[0].Message)
	}
	r := resp.Data.StagedUploadsCreate
	if len(r.UserErrors) > 0 {
		return nil, fmt.Errorf("shopify stagedUpload: %s", r.UserErrors[0].Message)
	}
	if len(r.StagedTargets) == 0 {
		return nil, fmt.Errorf("shopify stagedUpload: no targets returned")
	}
	return toStagedUploadTarget(r.StagedTargets[0]), nil
}

func toStagedUploadTarget(t gqlStagedTarget) *StagedUploadTarget {
	params := make([]StagedUploadParam, len(t.Parameters))
	for i, p := range t.Parameters {
		params[i] = StagedUploadParam{Name: p.Name, Value: p.Value}
	}
	return &StagedUploadTarget{
		URL: t.URL, ResourceURL: t.ResourceURL, Parameters: params,
	}
}

// --- UploadToStagedTarget ---

// UploadToStagedTarget uploads file data to a Shopify staged upload URL via multipart POST.
func (c *ShopifyClient) UploadToStagedTarget(ctx context.Context, target StagedUploadTarget, data []byte) error {
	body, boundary, err := buildMultipartBody(target.Parameters, data)
	if err != nil {
		return fmt.Errorf("build staged upload body: %w", err)
	}
	return postMultipart(ctx, target.URL, body, boundary)
}

func buildMultipartBody(params []StagedUploadParam, data []byte) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writeFormParams(writer, params); err != nil {
		return nil, "", err
	}
	if err := writeFilePart(writer, data); err != nil {
		return nil, "", err
	}
	boundary := writer.Boundary()
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body, boundary, nil
}

func writeFormParams(w *multipart.Writer, params []StagedUploadParam) error {
	for _, p := range params {
		if err := w.WriteField(p.Name, p.Value); err != nil {
			return err
		}
	}
	return nil
}

func writeFilePart(w *multipart.Writer, data []byte) error {
	part, err := w.CreateFormFile("file", "image")
	if err != nil {
		return err
	}
	_, err = io.Copy(part, bytes.NewReader(data))
	return err
}

func postMultipart(ctx context.Context, url string, body *bytes.Buffer, boundary string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	resp, err := httpclient.Default.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("staged upload failed: status %d", resp.StatusCode)
	}
	return nil
}

// --- AssociateMedia ---

// AssociateMedia attaches media items to a Shopify product via the productCreateMedia mutation.
func (c *ShopifyClient) AssociateMedia(ctx context.Context, creds ShopifyCreds, shopifyProductID string, media []MediaInput) error {
	body, err := buildCreateMediaBody(shopifyProductID, media)
	if err != nil {
		return err
	}
	resp, err := decodeMediaGQL[createMediaGQLResponse](ctx, creds, body)
	if err != nil {
		return err
	}
	return checkCreateMediaErrors(*resp)
}

func buildCreateMediaBody(productID string, media []MediaInput) ([]byte, error) {
	inputs := toGQLMediaInputs(media)
	vars := createMediaVars{ProductID: productID, Media: inputs}
	return marshalMediaGQLBody(productCreateMediaMutation, vars)
}

func toGQLMediaInputs(media []MediaInput) []gqlCreateMediaInput {
	inputs := make([]gqlCreateMediaInput, len(media))
	for i, m := range media {
		inputs[i] = gqlCreateMediaInput{
			OriginalSource:   m.OriginalSource,
			Alt:              m.Alt,
			MediaContentType: m.MediaContentType,
		}
	}
	return inputs
}

func checkCreateMediaErrors(resp createMediaGQLResponse) error {
	if len(resp.Errors) > 0 {
		return fmt.Errorf("shopify createMedia: %s", resp.Errors[0].Message)
	}
	errs := resp.Data.ProductCreateMedia.MediaUserErrors
	if len(errs) > 0 {
		return fmt.Errorf("shopify createMedia: %s", errs[0].Message)
	}
	return nil
}

// --- DeleteMedia ---

// DeleteMedia removes media items from a Shopify product.
func (c *ShopifyClient) DeleteMedia(ctx context.Context, creds ShopifyCreds, productID string, mediaIDs []string) error {
	body, err := buildDeleteMediaBody(productID, mediaIDs)
	if err != nil {
		return err
	}
	resp, err := decodeMediaGQL[deleteMediaGQLResponse](ctx, creds, body)
	if err != nil {
		return err
	}
	return checkDeleteMediaErrors(*resp)
}

func buildDeleteMediaBody(productID string, mediaIDs []string) ([]byte, error) {
	vars := deleteMediaVars{ProductID: productID, MediaIDs: mediaIDs}
	return marshalMediaGQLBody(productDeleteMediaMutation, vars)
}

func checkDeleteMediaErrors(resp deleteMediaGQLResponse) error {
	if len(resp.Errors) > 0 {
		return fmt.Errorf("shopify deleteMedia: %s", resp.Errors[0].Message)
	}
	errs := resp.Data.ProductDeleteMedia.MediaUserErrors
	if len(errs) > 0 {
		return fmt.Errorf("shopify deleteMedia: %s", errs[0].Message)
	}
	return nil
}

// --- ReorderMedia ---

// ReorderMedia reorders media items on a Shopify product.
func (c *ShopifyClient) ReorderMedia(ctx context.Context, creds ShopifyCreds, productID string, moves []MoveInput) error {
	body, err := buildReorderMediaBody(productID, moves)
	if err != nil {
		return err
	}
	resp, err := decodeMediaGQL[reorderMediaGQLResponse](ctx, creds, body)
	if err != nil {
		return err
	}
	return checkReorderMediaErrors(*resp)
}

func buildReorderMediaBody(productID string, moves []MoveInput) ([]byte, error) {
	vars := reorderMediaVars{ID: productID, Moves: moves}
	return marshalMediaGQLBody(productReorderMediaMutation, vars)
}

func checkReorderMediaErrors(resp reorderMediaGQLResponse) error {
	if len(resp.Errors) > 0 {
		return fmt.Errorf("shopify reorderMedia: %s", resp.Errors[0].Message)
	}
	errs := resp.Data.ProductReorderMedia.MediaUserErrors
	if len(errs) > 0 {
		return fmt.Errorf("shopify reorderMedia: %s", errs[0].Message)
	}
	return nil
}

// --- Generic Helpers ---

func decodeMediaGQL[T stagedUploadGQLResponse | createMediaGQLResponse | deleteMediaGQLResponse | reorderMediaGQLResponse](ctx context.Context, creds ShopifyCreds, body []byte) (*T, error) {
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

func marshalMediaGQLBody[V stagedUploadVars | createMediaVars | deleteMediaVars | reorderMediaVars](query string, vars V) ([]byte, error) {
	type body struct {
		Query     string `json:"query"`
		Variables V      `json:"variables,omitempty"`
	}
	return json.Marshal(body{Query: query, Variables: vars})
}

// --- GQL Mutations ---

const stagedUploadsCreateMutation = `mutation($input: [StagedUploadInput!]!) {
  stagedUploadsCreate(input: $input) {
    stagedTargets {
      url
      resourceUrl
      parameters { name value }
    }
    userErrors { field message }
  }
}`

const productCreateMediaMutation = `mutation($productId: ID!, $media: [CreateMediaInput!]!) {
  productCreateMedia(productId: $productId, media: $media) {
    media {
      ... on MediaImage { id }
    }
    mediaUserErrors { field message }
  }
}`

const productDeleteMediaMutation = `mutation($productId: ID!, $mediaIds: [ID!]!) {
  productDeleteMedia(productId: $productId, mediaIds: $mediaIds) {
    deletedMediaIds
    mediaUserErrors { field message }
  }
}`

const productReorderMediaMutation = `mutation($id: ID!, $moves: [MoveInput!]!) {
  productReorderMedia(id: $id, moves: $moves) {
    job { id }
    mediaUserErrors { field message }
  }
}`
