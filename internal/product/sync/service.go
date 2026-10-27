package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"storr-api/internal/product"
	"storr-api/internal/secrets"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// Service orchestrates bi-directional product synchronization with Shopify.
type Service struct {
	repo      *product.Repository
	shopify   *ShopifyClient
	secrets   *secrets.SecretsClient
	imageDeps *ImageSyncDeps
}

// NewService creates a sync service with the required dependencies.
func NewService(repo *product.Repository, shopify *ShopifyClient, secrets *secrets.SecretsClient, imageDeps *ImageSyncDeps) *Service {
	return &Service{repo: repo, shopify: shopify, secrets: secrets, imageDeps: imageDeps}
}

// PullFromShopify fetches all products from Shopify and upserts them into the local store.
func (s *Service) PullFromShopify(ctx context.Context, userID, storeID string) (*product.SyncResult, error) {
	creds, err := s.loadCreds(ctx, userID, storeID)
	if err != nil {
		return nil, err
	}
	return s.pullAllPages(ctx, creds, storeID)
}

// PushToShopify pushes a single product to Shopify and updates its sync metadata.
func (s *Service) PushToShopify(ctx context.Context, userID, storeID, productID string) error {
	creds, err := s.loadCreds(ctx, userID, storeID)
	if err != nil {
		return err
	}
	p, err := s.repo.Get(ctx, storeID, productID)
	if err != nil {
		return err
	}
	return s.pushProduct(ctx, creds, *p)
}

// PushAllToShopify pushes all products in the store to Shopify.
func (s *Service) PushAllToShopify(ctx context.Context, userID, storeID string) (*product.SyncResult, error) {
	creds, err := s.loadCreds(ctx, userID, storeID)
	if err != nil {
		return nil, err
	}
	return s.pushAllProducts(ctx, creds, storeID)
}

// FetchShopifyVersion fetches the current Shopify version of a product for conflict comparison.
func (s *Service) FetchShopifyVersion(ctx context.Context, userID, storeID, productID string) (*product.Product, error) {
	creds, err := s.loadCreds(ctx, userID, storeID)
	if err != nil {
		return nil, err
	}
	p, err := s.repo.Get(ctx, storeID, productID)
	if err != nil {
		return nil, err
	}
	if p.ShopifyID == "" {
		return nil, fmt.Errorf("%w: product has no Shopify ID", product.ErrSyncFailed)
	}
	return s.shopify.FetchProduct(ctx, creds, p.ShopifyID)
}

// ResolveKeepLocal pushes the local version to Shopify and clears the conflict.
func (s *Service) ResolveKeepLocal(ctx context.Context, userID, storeID, productID string) error {
	return s.PushToShopify(ctx, userID, storeID, productID)
}

// ResolveUseShopify overwrites the local product with the current Shopify version.
func (s *Service) ResolveUseShopify(ctx context.Context, userID, storeID, productID string) error {
	creds, err := s.loadCreds(ctx, userID, storeID)
	if err != nil {
		return err
	}
	return s.overwriteFromShopify(ctx, creds, storeID, productID)
}

func (s *Service) overwriteFromShopify(ctx context.Context, creds ShopifyCreds, storeID, productID string) error {
	existing, err := s.repo.Get(ctx, storeID, productID)
	if err != nil {
		return err
	}
	if existing.ShopifyID == "" {
		return fmt.Errorf("%w: product has no Shopify ID", product.ErrSyncFailed)
	}
	shopifyVersion, err := s.shopify.FetchProduct(ctx, creds, existing.ShopifyID)
	if err != nil {
		return err
	}
	return s.updateFromPull(ctx, *existing, *shopifyVersion)
}

func (s *Service) loadCreds(ctx context.Context, userID, storeID string) (ShopifyCreds, error) {
	name := fmt.Sprintf(secrets.ShopifySecretNameFmt, userID, storeID)
	secret, err := secrets.GetSecret[secrets.ShopifySecret](s.secrets, ctx, name)
	if err != nil {
		return ShopifyCreds{}, fmt.Errorf("load shopify credentials: %w", err)
	}
	return ShopifyCreds{AdminToken: secret.AdminToken, Domain: secret.StoreDomain}, nil
}

// --- Pull helpers ---

func (s *Service) pullAllPages(ctx context.Context, creds ShopifyCreds, storeID string) (*product.SyncResult, error) {
	result := &product.SyncResult{}
	cursor := ""
	for {
		page, err := s.shopify.FetchProducts(ctx, creds, cursor)
		if err != nil {
			return nil, fmt.Errorf("%w: fetch products: %v", product.ErrSyncFailed, err)
		}
		s.processPulledPage(ctx, storeID, page.Products, result)
		if !page.HasNext {
			break
		}
		cursor = page.Cursor
	}
	return result, nil
}

func (s *Service) processPulledPage(ctx context.Context, storeID string, products []product.Product, result *product.SyncResult) {
	for _, p := range products {
		created, err := s.upsertPulledProduct(ctx, storeID, p)
		if err != nil {
			slog.Error("failed to sync product", "shopifyId", p.ShopifyID, "error", err)
			result.Failed++
			result.Errors = append(result.Errors, product.SyncError{ProductID: p.ShopifyID, Message: "failed to sync product"})
			continue
		}
		if created {
			result.Created++
		} else {
			result.Updated++
		}
	}
}

func (s *Service) upsertPulledProduct(ctx context.Context, storeID string, pulled product.Product) (bool, error) {
	existing, err := s.repo.GetByShopifyID(ctx, storeID, pulled.ShopifyID)
	if err != nil && !errors.Is(err, product.ErrProductNotFound) {
		return false, err
	}
	if existing != nil {
		return false, s.updateFromPull(ctx, *existing, pulled)
	}
	return true, s.createFromPull(ctx, storeID, pulled)
}

func (s *Service) updateFromPull(ctx context.Context, existing product.Product, pulled product.Product) error {
	now := time.Now().UTC()
	updated := product.ApplyPulledFields(existing, pulled)
	updated.LastPulledAt = &now
	updated.SyncStatus = product.SyncSynced
	updated.UpdatedAt = now
	return s.repo.Update(ctx, updated)
}

func (s *Service) createFromPull(ctx context.Context, storeID string, pulled product.Product) error {
	now := time.Now().UTC()
	pulled.StoreID = storeID
	pulled.ProductID = product.ProductIDPrefix + uuid.NewString()
	pulled.LastPulledAt = &now
	pulled.SyncStatus = product.SyncSynced
	pulled.CreatedAt = now
	pulled.UpdatedAt = now
	return s.repo.Create(ctx, pulled)
}

// --- Push helpers ---

func (s *Service) pushAllProducts(ctx context.Context, creds ShopifyCreds, storeID string) (*product.SyncResult, error) {
	result := &product.SyncResult{}
	cursor := ""
	for {
		listResult, err := s.repo.List(ctx, storeID, cursor, product.MaxPageSize, product.PageForward)
		if err != nil {
			return nil, fmt.Errorf("%w: list products: %v", product.ErrSyncFailed, err)
		}
		s.pushProductBatch(ctx, creds, listResult.Products, result)
		if listResult.NextCursor == "" {
			break
		}
		cursor = listResult.NextCursor
	}
	return result, nil
}

func (s *Service) pushProductBatch(ctx context.Context, creds ShopifyCreds, products []product.Product, result *product.SyncResult) {
	for _, p := range products {
		isNew := p.ShopifyID == ""
		if err := s.pushProduct(ctx, creds, p); err != nil {
			slog.Error("failed to push product", "productId", p.ProductID, "error", err)
			result.Failed++
			result.Errors = append(result.Errors, product.SyncError{ProductID: p.ProductID, Message: "failed to push product"})
			continue
		}
		if isNew {
			result.Created++
		} else {
			result.Updated++
		}
	}
}

func (s *Service) pushProduct(ctx context.Context, creds ShopifyCreds, p product.Product) error {
	input := StorrProductToShopifyInput(p)
	shopifyID, err := s.shopify.UpsertProduct(ctx, creds, input)
	if err != nil {
		return fmt.Errorf("push to shopify: %w", err)
	}
	if err := s.pushProductImages(ctx, creds, shopifyID, p.Images); err != nil {
		return fmt.Errorf("push images: %w", err)
	}
	if err := s.setPublicationState(ctx, creds, shopifyID, p.Published); err != nil {
		return fmt.Errorf("set publication: %w", err)
	}
	p = s.swapImageURLs(ctx, creds, shopifyID, p)
	return s.markProductPushed(ctx, p, shopifyID)
}

func (s *Service) setPublicationState(ctx context.Context, creds ShopifyCreds, shopifyID string, published bool) error {
	pubID, err := s.shopify.FetchOnlineStorePublicationID(ctx, creds)
	if err != nil {
		slog.Warn("fetch publication ID failed, skipping publish", "error", err)
		return nil
	}
	if pubID == "" {
		return nil
	}
	if published {
		return s.shopify.PublishProduct(ctx, creds, shopifyID, pubID)
	}
	return s.shopify.UnpublishProduct(ctx, creds, shopifyID, pubID)
}

func (s *Service) markProductPushed(ctx context.Context, p product.Product, shopifyID string) error {
	now := time.Now().UTC()
	p.ShopifyID = shopifyID
	p.LastPushedAt = &now
	p.SyncStatus = product.SyncSynced
	p.UpdatedAt = now
	return s.repo.Update(ctx, p)
}

// --- Image push helpers ---

func (s *Service) pushProductImages(ctx context.Context, creds ShopifyCreds, shopifyID string, images []product.Image) error {
	local := filterLocalImages(images)
	if len(local) == 0 || s.imageDeps == nil {
		return nil
	}
	media, err := s.buildMediaInputs(ctx, local)
	if err != nil {
		return err
	}
	return s.shopify.AssociateMedia(ctx, creds, shopifyID, media)
}

func (s *Service) buildMediaInputs(ctx context.Context, images []product.Image) ([]MediaInput, error) {
	media := make([]MediaInput, 0, len(images))
	for _, img := range images {
		input, err := s.buildMediaInput(ctx, img)
		if err != nil {
			return nil, err
		}
		media = append(media, input)
	}
	return media, nil
}

func (s *Service) buildMediaInput(ctx context.Context, img product.Image) (MediaInput, error) {
	url, err := s.presignImageGet(ctx, img.URL)
	if err != nil {
		return MediaInput{}, fmt.Errorf("presign image: %w", err)
	}
	return MediaInput{
		OriginalSource: url, Alt: img.Name, MediaContentType: "IMAGE",
	}, nil
}

func (s *Service) presignImageGet(ctx context.Context, imageURL string) (string, error) {
	key := extractS3Key(imageURL, s.imageDeps.Bucket)
	expiry := time.Duration(PresignGetExpirySec) * time.Second
	result, err := s.imageDeps.Presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.imageDeps.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func filterLocalImages(images []product.Image) []product.Image {
	var local []product.Image
	for _, img := range images {
		if product.IsLocalImageURL(img.URL) {
			local = append(local, img)
		}
	}
	return local
}

// --- Image URL swap (S3 -> Shopify CDN) ---

func (s *Service) swapImageURLs(ctx context.Context, creds ShopifyCreds, shopifyID string, p product.Product) product.Product {
	localImages := filterLocalImages(p.Images)
	if len(localImages) == 0 {
		return p
	}
	shopifyProduct, err := s.shopify.FetchProduct(ctx, creds, shopifyID)
	if err != nil {
		slog.Warn("failed to fetch product for image swap", "error", err)
		return p
	}
	cdnImages := findNewShopifyImages(p.Images, shopifyProduct.Images)
	if len(cdnImages) == 0 {
		return p
	}
	updatedImages, s3Keys := applyImageSwap(p.Images, localImages, cdnImages)
	p.Images = updatedImages
	s.deleteS3Objects(ctx, s3Keys)
	return p
}

// findNewShopifyImages returns Shopify images whose GIDs are not already tracked locally.
func findNewShopifyImages(localImages, shopifyImages []product.Image) []product.Image {
	known := make(map[string]bool, len(localImages))
	for _, img := range localImages {
		if product.IsSyncedImage(img) {
			known[img.ImageID] = true
		}
	}
	var newImages []product.Image
	for _, img := range shopifyImages {
		if !known[img.ImageID] {
			newImages = append(newImages, img)
		}
	}
	return newImages
}

// applyImageSwap replaces local S3 images with their CDN counterparts matched by order,
// and returns the updated image list and S3 keys of the replaced images for cleanup.
func applyImageSwap(images []product.Image, localImages, cdnImages []product.Image) ([]product.Image, []string) {
	result := make([]product.Image, len(images))
	copy(result, images)
	var s3Keys []string
	cdnIdx := 0
	for i := range result {
		if cdnIdx >= len(cdnImages) {
			break
		}
		if !product.IsLocalImageURL(result[i].URL) {
			continue
		}
		s3Keys = append(s3Keys, extractS3KeyFromURL(result[i].URL))
		result[i] = applyShopifyCDNImage(result[i], cdnImages[cdnIdx])
		cdnIdx++
	}
	return result, s3Keys
}

func applyShopifyCDNImage(local product.Image, cdn product.Image) product.Image {
	local.ImageID = cdn.ImageID
	local.URL = cdn.URL
	local.Width = cdn.Width
	local.Height = cdn.Height
	return local
}

func extractS3KeyFromURL(imageURL string) string {
	// Extract key from URL like https://bucket.s3.amazonaws.com/stores/...
	idx := strings.Index(imageURL, ".s3.amazonaws.com/")
	if idx == -1 {
		return ""
	}
	return imageURL[idx+len(".s3.amazonaws.com/"):]
}

func extractS3Key(imageURL, bucket string) string {
	prefix := fmt.Sprintf("https://%s.s3.amazonaws.com/", bucket)
	return strings.TrimPrefix(imageURL, prefix)
}


func (s *Service) deleteS3Object(ctx context.Context, key string) {
	if s.imageDeps == nil || s.imageDeps.S3 == nil {
		return
	}
	_, err := s.imageDeps.S3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.imageDeps.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		slog.Warn("failed to delete S3 object", "key", key, "error", err)
	}
}

func (s *Service) deleteS3Objects(ctx context.Context, keys []string) {
	for _, key := range keys {
		s.deleteS3Object(ctx, key)
	}
}

// --- Shopify-aware image operations ---

// DeleteShopifyImage deletes a synced image from the Shopify product.
func (s *Service) DeleteShopifyImage(ctx context.Context, op product.ShopifyImageOp, img product.Image) error {
	if op.Product.ShopifyID == "" || !product.IsSyncedImage(img) {
		return nil
	}
	creds, err := s.loadCreds(ctx, op.UserID, op.StoreID)
	if err != nil {
		return err
	}
	return s.shopify.DeleteMedia(ctx, creds, op.Product.ShopifyID, []string{img.ImageID})
}

// ReorderShopifyImages reorders synced images on the Shopify product.
func (s *Service) ReorderShopifyImages(ctx context.Context, op product.ShopifyImageOp, entries []product.ReorderImageEntry) error {
	if op.Product.ShopifyID == "" {
		return nil
	}
	moves := buildShopifyMoves(op.Product.Images, entries)
	if len(moves) == 0 {
		return nil
	}
	creds, err := s.loadCreds(ctx, op.UserID, op.StoreID)
	if err != nil {
		return err
	}
	return s.shopify.ReorderMedia(ctx, creds, op.Product.ShopifyID, moves)
}

// buildShopifyMoves filters to synced images and converts Storr positions (1-indexed) to Shopify positions (0-indexed).
func buildShopifyMoves(images []product.Image, entries []product.ReorderImageEntry) []MoveInput {
	imageMap := make(map[string]product.Image, len(images))
	for _, img := range images {
		imageMap[img.ImageID] = img
	}
	var moves []MoveInput
	for _, e := range entries {
		img, ok := imageMap[e.ImageID]
		if !ok || !product.IsSyncedImage(img) {
			continue
		}
		moves = append(moves, MoveInput{
			ID:          img.ImageID,
			NewPosition: strconv.Itoa(e.Position - 1),
		})
	}
	return moves
}
