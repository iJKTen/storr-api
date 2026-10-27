package webhook

import (
	"fmt"

	"storr-api/internal/product"
)

func webhookProductToStorr(wp webhookProduct) product.Product {
	return product.Product{
		ShopifyID:      shopifyProductGID(wp.ID),
		Handle:         wp.Handle,
		Title:          wp.Title,
		BodyHTML:       wp.BodyHTML,
		Vendor:         wp.Vendor,
		ProductType:    wp.ProductType,
		Tags:           product.SplitTags(wp.Tags),
		Status:         product.ShopifyStatusToStorr(wp.Status),
		TemplateSuffix: wp.TemplateSuffix,
		PublishedAt:    product.ParseShopifyTime(wp.PublishedAt),
		Options:        convertWebhookOptions(wp.Options),
		Variants:       convertWebhookVariants(wp.Variants),
		Images:         convertWebhookImages(wp.Images),
		SyncStatus:     product.SyncSynced,
	}
}

func convertWebhookOptions(opts []webhookOption) []product.Option {
	result := make([]product.Option, len(opts))
	for i, o := range opts {
		result[i] = product.Option{Name: o.Name, Position: o.Position, Values: o.Values}
	}
	return result
}

func convertWebhookVariants(variants []webhookVariant) []product.Variant {
	result := make([]product.Variant, len(variants))
	for i, v := range variants {
		result[i] = webhookVariantToStorr(v)
	}
	return result
}

func webhookVariantToStorr(v webhookVariant) product.Variant {
	out := buildBaseVariant(v)
	if v.CompareAtPrice != nil {
		out.CompareAtPrice = *v.CompareAtPrice
	}
	return out
}

func buildBaseVariant(v webhookVariant) product.Variant {
	return product.Variant{
		ShopifyVariantID:   shopifyVariantGID(v.ID),
		Title:              v.Title,
		SKU:                v.SKU,
		Barcode:            v.Barcode,
		Price:              v.Price,
		Position:           v.Position,
		InventoryQty:       v.InventoryQuantity,
		InventoryPolicy:    v.InventoryPolicy,
		FulfillmentService: v.FulfillmentService,
		Option1:            v.Option1,
		Option2:            v.Option2,
		Option3:            v.Option3,
		Taxable:            v.Taxable,
		RequiresShipping:   v.RequiresShipping,
		Weight:             v.Weight,
		WeightUnit:         v.WeightUnit,
		Grams:              v.Grams,
	}
}

func convertWebhookImages(images []webhookImage) []product.Image {
	result := make([]product.Image, len(images))
	for i, img := range images {
		alt := ""
		if img.Alt != nil {
			alt = *img.Alt
		}
		result[i] = product.Image{
			ImageID:  shopifyImageGID(img.ID),
			URL:      img.Src,
			AltText:  alt,
			Position: img.Position,
			Width:    img.Width,
			Height:   img.Height,
		}
	}
	return result
}

func shopifyProductGID(id int64) string {
	return fmt.Sprintf(product.ShopifyProductGIDFmt, id)
}

func shopifyVariantGID(id int64) string {
	return fmt.Sprintf(product.ShopifyVariantGIDFmt, id)
}

func shopifyImageGID(id int64) string {
	return fmt.Sprintf(product.ShopifyImageGIDFmt, id)
}

