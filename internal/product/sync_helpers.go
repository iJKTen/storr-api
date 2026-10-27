package product

// ApplyPulledFields copies Shopify-managed fields from a pulled product onto
// an existing local product, preserving local-only metadata (IDs, timestamps,
// sync status). Used by both the sync service and the webhook service.
func ApplyPulledFields(existing Product, pulled Product) Product {
	existing.Handle = pulled.Handle
	existing.Title = pulled.Title
	existing.BodyHTML = pulled.BodyHTML
	existing.Vendor = pulled.Vendor
	existing.ProductType = pulled.ProductType
	existing.Tags = pulled.Tags
	existing.Status = pulled.Status
	existing.TemplateSuffix = pulled.TemplateSuffix
	existing.Options = pulled.Options
	existing.Variants = pulled.Variants
	existing.Images = pulled.Images
	return existing
}
