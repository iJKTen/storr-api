package image

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"storr-api/internal/product"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// Service manages product image operations including S3 presigned uploads.
type Service struct {
	repo    *product.Repository
	presign *s3.PresignClient
	s3      *s3.Client
	bucket  string
}

// NewService creates an image service backed by the given repository and S3 presign client.
func NewService(repo *product.Repository, presign *s3.PresignClient, s3Client *s3.Client, bucket string) *Service {
	return &Service{repo: repo, presign: presign, s3: s3Client, bucket: bucket}
}

// GenerateUploadURL creates a presigned S3 PUT URL for direct client-side image upload.
func (s *Service) GenerateUploadURL(ctx context.Context, storeID, productID string, req product.UploadURLRequest) (*product.UploadURLResponse, error) {
	if err := validateUploadRequest(req); err != nil {
		return nil, err
	}
	imageKey := buildImageKey(storeID, productID, req.FileName)
	url, err := s.presignPut(ctx, imageKey, req.ContentType)
	if err != nil {
		return nil, fmt.Errorf("generate upload url: %w", err)
	}
	return &product.UploadURLResponse{UploadURL: url, ImageKey: imageKey}, nil
}

// ConfirmUpload adds a confirmed image to the product's image list.
func (s *Service) ConfirmUpload(ctx context.Context, storeID, productID string, req product.ConfirmImageRequest) (*product.Product, error) {
	if err := validateImageKey(req.ImageKey, storeID, productID); err != nil {
		return nil, err
	}
	p, err := s.repo.Get(ctx, storeID, productID)
	if err != nil {
		return nil, err
	}
	img := buildImageFromConfirm(req, s.bucket)
	p.Images = append(p.Images, img)
	p.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, *p); err != nil {
		return nil, fmt.Errorf("confirm image: %w", err)
	}
	return p, nil
}

// ImageRef identifies a specific image within a store's product.
type ImageRef struct {
	StoreID   string
	ProductID string
	ImageID   string
}

// UpdateImage modifies image properties (name, altText, position).
func (s *Service) UpdateImage(ctx context.Context, ref ImageRef, req product.UpdateImageRequest) (*product.Product, error) {
	p, err := s.repo.Get(ctx, ref.StoreID, ref.ProductID)
	if err != nil {
		return nil, err
	}
	idx := findImageIndex(p.Images, ref.ImageID)
	if idx == -1 {
		return nil, fmt.Errorf("%w: image not found", product.ErrProductNotFound)
	}
	applyImageUpdate(&p.Images[idx], req)
	p.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, *p); err != nil {
		return nil, fmt.Errorf("update image: %w", err)
	}
	return p, nil
}

// DeleteImage removes an image from the product and cleans up S3 if applicable.
// Returns the product and the deleted image so the caller can propagate to Shopify if needed.
func (s *Service) DeleteImage(ctx context.Context, storeID, productID, imageID string) (*product.Product, *product.Image, error) {
	p, err := s.repo.Get(ctx, storeID, productID)
	if err != nil {
		return nil, nil, err
	}
	idx := findImageIndex(p.Images, imageID)
	if idx == -1 {
		return nil, nil, fmt.Errorf("%w: image not found", product.ErrProductNotFound)
	}
	deleted := p.Images[idx]
	if product.IsLocalImageURL(deleted.URL) {
		s.deleteS3Object(ctx, deleted.URL)
	}
	p.Images = removeImageAt(p.Images, idx)
	p.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, *p); err != nil {
		return nil, nil, fmt.Errorf("delete image: %w", err)
	}
	return p, &deleted, nil
}

// ReorderImages updates the position of each image specified in the entries.
func (s *Service) ReorderImages(ctx context.Context, storeID, productID string, entries []product.ReorderImageEntry) (*product.Product, error) {
	p, err := s.repo.Get(ctx, storeID, productID)
	if err != nil {
		return nil, err
	}
	p.Images = applyImageReorder(p.Images, entries)
	p.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, *p); err != nil {
		return nil, fmt.Errorf("reorder images: %w", err)
	}
	return p, nil
}

func (s *Service) presignPut(ctx context.Context, key, contentType string) (string, error) {
	expiry := time.Duration(product.PresignedURLExpirySec) * time.Second
	result, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func validateUploadRequest(req product.UploadURLRequest) error {
	if req.FileName == "" {
		return fmt.Errorf("%w: fileName is required", product.ErrInvalidProductData)
	}
	if req.ContentType == "" {
		return fmt.Errorf("%w: contentType is required", product.ErrInvalidProductData)
	}
	if !allowedImageTypes[req.ContentType] {
		return fmt.Errorf("%w: unsupported content type", product.ErrInvalidProductData)
	}
	return nil
}

// allowedImageTypes is the set of MIME types permitted for product image uploads.
var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

// validateImageKey ensures the submitted key belongs to the expected store/product
// path and contains no path traversal sequences.
func validateImageKey(imageKey, storeID, productID string) error {
	if strings.Contains(imageKey, "..") {
		return fmt.Errorf("%w: invalid image key", product.ErrInvalidProductData)
	}
	expectedPrefix := fmt.Sprintf("%s/%s/%s/%s", product.ImageKeyPrefix, storeID, productID, product.ImageIDPrefix)
	if !strings.HasPrefix(imageKey, expectedPrefix) {
		return fmt.Errorf("%w: image key does not match expected path", product.ErrInvalidProductData)
	}
	return nil
}

func buildImageKey(storeID, productID, fileName string) string {
	ext := filepath.Ext(fileName)
	return fmt.Sprintf("%s/%s/%s/%s%s%s", product.ImageKeyPrefix, storeID, productID, product.ImageIDPrefix, uuid.NewString(), ext)
}

func buildImageFromConfirm(req product.ConfirmImageRequest, bucket string) product.Image {
	return product.Image{
		ImageID: product.ImageIDPrefix + uuid.NewString(),
		URL:     fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucket, req.ImageKey),
		Name:    req.Name,
		AltText: req.AltText,
	}
}

func findImageIndex(images []product.Image, imageID string) int {
	for i, img := range images {
		if img.ImageID == imageID {
			return i
		}
	}
	return -1
}

func applyImageUpdate(img *product.Image, req product.UpdateImageRequest) {
	img.Name = req.Name
	img.AltText = req.AltText
	img.Position = req.Position
}

func removeImageAt(images []product.Image, idx int) []product.Image {
	return append(images[:idx], images[idx+1:]...)
}

// deleteS3Object removes the S3 object backing a local image URL.
// Errors are logged but not returned because image deletion from the product
// record has already succeeded; orphaned S3 objects are cleaned up separately.
func (s *Service) deleteS3Object(ctx context.Context, imageURL string) {
	if s.s3 == nil {
		return
	}
	idx := strings.Index(imageURL, ".s3.amazonaws.com/")
	if idx == -1 {
		return
	}
	key := imageURL[idx+len(".s3.amazonaws.com/"):]
	_, err := s.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		slog.Warn("failed to delete S3 object", "key", key, "error", err)
	}
}

func applyImageReorder(images []product.Image, entries []product.ReorderImageEntry) []product.Image {
	result := make([]product.Image, len(images))
	copy(result, images)
	posMap := make(map[string]int, len(entries))
	for _, e := range entries {
		posMap[e.ImageID] = e.Position
	}
	for i := range result {
		if pos, ok := posMap[result[i].ImageID]; ok {
			result[i].Position = pos
		}
	}
	return result
}
