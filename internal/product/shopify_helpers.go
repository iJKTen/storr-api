package product

import (
	"strings"
	"time"
)

// IsLocalImageURL returns true if the URL points to an S3-hosted image.
func IsLocalImageURL(url string) bool {
	return strings.Contains(url, ".s3.amazonaws.com/")
}

// ShopifyStatusToStorr maps a Shopify product status string to the local ProductStatus enum.
func ShopifyStatusToStorr(s string) ProductStatus {
	switch strings.ToUpper(s) {
	case "ACTIVE":
		return StatusActive
	case "ARCHIVED":
		return StatusArchived
	default:
		return StatusDraft
	}
}

// ParseShopifyTime parses an optional RFC3339 timestamp from Shopify into a *time.Time.
func ParseShopifyTime(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil
	}
	return &t
}

// IsSyncedImage returns true if the image has a Shopify media GID as its ImageID.
func IsSyncedImage(img Image) bool {
	return strings.HasPrefix(img.ImageID, ShopifyMediaGIDPrefix)
}

// SplitTags splits a comma-separated tag string into trimmed, non-empty tag values.
func SplitTags(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
