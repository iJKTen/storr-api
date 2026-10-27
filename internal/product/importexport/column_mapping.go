package importexport

import "strings"

const (
	fieldHandle              = "handle"
	fieldTitle               = "title"
	fieldBodyHTML            = "bodyHtml"
	fieldVendor              = "vendor"
	fieldProductType         = "productType"
	fieldTags                = "tags"
	fieldStatus              = "status"
	fieldPublishedScope      = "publishedScope"
	fieldTemplateSuffix      = "templateSuffix"
	fieldPublished           = "published"
	fieldSEOTitle            = "seoTitle"
	fieldSEODesc             = "seoDescription"
	fieldGiftCard            = "giftCard"
	fieldOption1Name         = "option1Name"
	fieldOption1Value        = "option1Value"
	fieldOption2Name         = "option2Name"
	fieldOption2Value        = "option2Value"
	fieldOption3Name         = "option3Name"
	fieldOption3Value        = "option3Value"
	fieldVariantSKU          = "variantSku"
	fieldVariantBarcode      = "variantBarcode"
	fieldVariantPrice        = "variantPrice"
	fieldCompareAtPrice      = "variantCompareAtPrice"
	fieldCostPerItem         = "costPerItem"
	fieldInventoryQty        = "variantInventoryQty"
	fieldInventoryPolicy     = "variantInventoryPolicy"
	fieldInventoryManagement = "variantInventoryManagement"
	fieldFulfillmentService  = "variantFulfillmentService"
	fieldVariantTaxable      = "variantTaxable"
	fieldVariantTaxCode      = "variantTaxCode"
	fieldRequiresShipping    = "variantRequiresShipping"
	fieldVariantWeight       = "variantWeight"
	fieldVariantWeightUnit   = "variantWeightUnit"
	fieldVariantGrams        = "variantGrams"
	fieldImageSrc            = "imageSrc"
	fieldImageAlt            = "imageAlt"
	fieldImageName           = "imageName"
	metafieldPrefix          = "product.metafields."
)

const (
	defaultOptionName  = "Option1"
	defaultOption2Name = "Option2"
	defaultOption3Name = "Option3"
)

// ColumnMapping defines how a CSV/Excel column maps to a product field.
type ColumnMapping struct {
	FieldName   string   `json:"fieldName"`
	DisplayName string   `json:"displayName"`
	Aliases     []string `json:"aliases"`
	Required    bool     `json:"required"`
	Example     string   `json:"example"`
}

// MappingRegistry returns the full list of supported column mappings
// covering all Shopify product CSV fields.
func MappingRegistry() []ColumnMapping {
	return append(productMappings(), variantMappings()...)
}

func productMappings() []ColumnMapping {
	return []ColumnMapping{
		handleMapping(),
		titleMapping(),
		bodyMapping(),
		vendorMapping(),
		typeMapping(),
		tagsMapping(),
		statusMapping(),
		publishedMapping(),
		publishedScopeMapping(),
		templateSuffixMapping(),
		seoTitleMapping(),
		seoDescMapping(),
		giftCardMapping(),
	}
}

func publishedMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldPublished,
		DisplayName: "Published",
		Aliases:     []string{"published", "published on online store"},
		Required:    false,
		Example:     "true",
	}
}

func handleMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldHandle,
		DisplayName: "Handle",
		Aliases:     []string{"handle", "url-handle", "url_handle", "url handle", "slug"},
		Required:    true,
		Example:     "example-t-shirt",
	}
}

func titleMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldTitle,
		DisplayName: "Title",
		Aliases:     []string{"title", "name", "product-name", "product_name", "product-title", "product_title"},
		Required:    true,
		Example:     "Example T-Shirt",
	}
}

func bodyMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldBodyHTML,
		DisplayName: "Body (HTML)",
		Aliases:     []string{"body (html)", "body", "body-html", "body_html", "description"},
		Required:    false,
		Example:     "<p>Soft cotton tee</p>",
	}
}

func vendorMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldVendor,
		DisplayName: "Vendor",
		Aliases:     []string{"vendor", "brand", "manufacturer"},
		Required:    false,
		Example:     "Acme Co",
	}
}

func typeMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldProductType,
		DisplayName: "Type",
		Aliases:     []string{"type", "product-type", "product_type", "category"},
		Required:    false,
		Example:     "Shirts",
	}
}

func tagsMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldTags,
		DisplayName: "Tags",
		Aliases:     []string{"tags"},
		Required:    false,
		Example:     "summer, cotton, sale",
	}
}

func statusMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldStatus,
		DisplayName: "Status",
		Aliases:     []string{"status"},
		Required:    false,
		Example:     "active",
	}
}

func publishedScopeMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldPublishedScope,
		DisplayName: "Published Scope",
		Aliases:     []string{"published scope", "published_scope"},
		Required:    false,
		Example:     "global",
	}
}

func templateSuffixMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldTemplateSuffix,
		DisplayName: "Template Suffix",
		Aliases:     []string{"template suffix", "template_suffix"},
		Required:    false,
		Example:     "",
	}
}

func option1NameMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldOption1Name,
		DisplayName: "Option1 Name",
		Aliases:     []string{"option1 name", "option1_name", "option-1-name"},
		Required:    false,
		Example:     "Size",
	}
}

func variantMappings() []ColumnMapping {
	return []ColumnMapping{
		option1NameMapping(),
		option1ValueMapping(),
		option2NameMapping(),
		option2ValueMapping(),
		option3NameMapping(),
		option3ValueMapping(),
		skuMapping(),
		barcodeMapping(),
		priceMapping(),
		compareAtPriceMapping(),
		costPerItemMapping(),
		inventoryQtyMapping(),
		inventoryPolicyMapping(),
		inventoryMgmtMapping(),
		fulfillmentMapping(),
		taxableMapping(),
		taxCodeMapping(),
		shippingMapping(),
		weightMapping(),
		weightUnitMapping(),
		gramsMapping(),
		imageSrcMapping(),
		imageAltMapping(),
		imageNameMapping(),
	}
}

func option1ValueMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldOption1Value,
		DisplayName: "Option1 Value",
		Aliases:     []string{"option1 value", "option1_value", "option-1-value"},
		Required:    false,
		Example:     "Small",
	}
}

func option2NameMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldOption2Name,
		DisplayName: "Option2 Name",
		Aliases:     []string{"option2 name", "option2_name"},
		Required:    false,
		Example:     "Color",
	}
}

func option2ValueMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldOption2Value,
		DisplayName: "Option2 Value",
		Aliases:     []string{"option2 value", "option2_value"},
		Required:    false,
		Example:     "Red",
	}
}

func option3NameMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldOption3Name,
		DisplayName: "Option3 Name",
		Aliases:     []string{"option3 name", "option3_name"},
		Required:    false,
		Example:     "Material",
	}
}

func option3ValueMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldOption3Value,
		DisplayName: "Option3 Value",
		Aliases:     []string{"option3 value", "option3_value"},
		Required:    false,
		Example:     "Cotton",
	}
}

func seoTitleMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldSEOTitle,
		DisplayName: "SEO Title",
		Aliases:     []string{"seo title", "seo_title"},
		Required:    false,
		Example:     "Example T-Shirt | Store",
	}
}

func seoDescMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldSEODesc,
		DisplayName: "SEO Description",
		Aliases:     []string{"seo description", "seo_description"},
		Required:    false,
		Example:     "Shop our cotton t-shirt...",
	}
}

func giftCardMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldGiftCard,
		DisplayName: "Gift Card",
		Aliases:     []string{"gift card", "gift_card"},
		Required:    false,
		Example:     "false",
	}
}

func costPerItemMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldCostPerItem,
		DisplayName: "Cost Per Item",
		Aliases:     []string{"cost per item", "cost_per_item"},
		Required:    false,
		Example:     "15.00",
	}
}

func skuMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldVariantSKU,
		DisplayName: "Variant SKU",
		Aliases:     []string{"variant sku", "variant_sku", "sku"},
		Required:    false,
		Example:     "TSHIRT-S-BLK",
	}
}

func barcodeMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldVariantBarcode,
		DisplayName: "Variant Barcode",
		Aliases:     []string{"variant barcode", "variant_barcode", "barcode", "upc", "ean"},
		Required:    false,
		Example:     "123456789012",
	}
}

func priceMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldVariantPrice,
		DisplayName: "Variant Price",
		Aliases:     []string{"variant price", "variant_price", "price"},
		Required:    true,
		Example:     "29.99",
	}
}

func compareAtPriceMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldCompareAtPrice,
		DisplayName: "Variant Compare At Price",
		Aliases:     []string{"variant compare at price", "variant_compare_at_price", "compare-at-price", "compare-at price", "compare_at_price", "original-price", "original_price"},
		Required:    false,
		Example:     "39.99",
	}
}

func inventoryQtyMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldInventoryQty,
		DisplayName: "Variant Inventory Qty",
		Aliases:     []string{"variant inventory qty", "variant_inventory_qty", "inventory", "qty", "stock", "quantity", "inventory quantity"},
		Required:    false,
		Example:     "100",
	}
}

func inventoryPolicyMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldInventoryPolicy,
		DisplayName: "Variant Inventory Policy",
		Aliases:     []string{"variant inventory policy", "variant_inventory_policy", "inventory-policy", "inventory_policy", "continue selling when out of stock"},
		Required:    false,
		Example:     "deny",
	}
}

func inventoryMgmtMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldInventoryManagement,
		DisplayName: "Variant Inventory Tracker",
		Aliases:     []string{"variant inventory tracker", "variant_inventory_tracker", "inventory-management", "inventory_management", "inventory tracker", "shopify tracking"},
		Required:    false,
		Example:     "shopify",
	}
}

func fulfillmentMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldFulfillmentService,
		DisplayName: "Variant Fulfillment Service",
		Aliases:     []string{"variant fulfillment service", "variant_fulfillment_service", "fulfillment-service", "fulfillment_service", "fulfillment service"},
		Required:    false,
		Example:     "manual",
	}
}

func taxableMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldVariantTaxable,
		DisplayName: "Variant Taxable",
		Aliases:     []string{"variant taxable", "variant_taxable", "taxable", "charge tax"},
		Required:    false,
		Example:     "true",
	}
}

func taxCodeMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldVariantTaxCode,
		DisplayName: "Variant Tax Code",
		Aliases:     []string{"variant tax code", "variant_tax_code", "tax-code", "tax_code"},
		Required:    false,
		Example:     "",
	}
}

func shippingMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldRequiresShipping,
		DisplayName: "Variant Requires Shipping",
		Aliases:     []string{"variant requires shipping", "variant_requires_shipping", "requires-shipping", "requires_shipping", "requires shipping"},
		Required:    false,
		Example:     "true",
	}
}

func weightMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldVariantWeight,
		DisplayName: "Variant Weight",
		Aliases:     []string{"variant weight", "variant_weight", "weight"},
		Required:    false,
		Example:     "0.5",
	}
}

func weightUnitMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldVariantWeightUnit,
		DisplayName: "Variant Weight Unit",
		Aliases:     []string{"variant weight unit", "variant_weight_unit", "weight-unit", "weight_unit", "weight unit for display"},
		Required:    false,
		Example:     "lb",
	}
}

func gramsMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldVariantGrams,
		DisplayName: "Variant Grams",
		Aliases:     []string{"variant grams", "variant_grams", "grams", "weight value (grams)"},
		Required:    false,
		Example:     "200",
	}
}

func imageSrcMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldImageSrc,
		DisplayName: "Image Src",
		Aliases:     []string{"image src", "image_src", "image", "image-url", "image_url", "product image url"},
		Required:    false,
		Example:     "https://cdn.example.com/tshirt.png",
	}
}

func imageAltMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldImageAlt,
		DisplayName: "Image Alt Text",
		Aliases:     []string{"image alt text", "image_alt_text", "image-alt", "image_alt", "alt-text", "alt_text"},
		Required:    false,
		Example:     "Black t-shirt front view",
	}
}

func imageNameMapping() ColumnMapping {
	return ColumnMapping{
		FieldName:   fieldImageName,
		DisplayName: "Image Name",
		Aliases:     []string{"image name", "image_name"},
		Required:    false,
		Example:     "black.front",
	}
}

// DetectMappings matches CSV/Excel header names against the mapping registry
// and returns a map of header -> fieldName for each matched column.
// Metafield columns are detected by the product.metafields prefix.
func DetectMappings(headers []string) map[string]string {
	registry := MappingRegistry()
	index := buildAliasIndex(registry)
	result := make(map[string]string, len(headers))
	for _, header := range headers {
		normalized := normalizeHeader(header)
		if fieldName, ok := index[normalized]; ok {
			result[header] = fieldName
			continue
		}
		if key := parseMetafieldKey(header); key != "" {
			result[header] = metafieldPrefix + key
		}
	}
	return result
}

func parseMetafieldKey(header string) string {
	lower := strings.ToLower(header)
	start := strings.Index(lower, metafieldPrefix)
	if start < 0 {
		return ""
	}
	key := header[start+len(metafieldPrefix):]
	key = strings.TrimRight(key, ")")
	key = strings.TrimSpace(key)
	if !strings.Contains(key, ".") {
		return ""
	}
	return key
}

func buildAliasIndex(mappings []ColumnMapping) map[string]string {
	index := make(map[string]string)
	for _, m := range mappings {
		for _, alias := range m.Aliases {
			index[normalizeHeader(alias)] = m.FieldName
		}
		index[normalizeHeader(m.DisplayName)] = m.FieldName
	}
	return index
}

func normalizeHeader(s string) string {
	trimmed := strings.TrimSpace(s)
	lower := strings.ToLower(trimmed)
	noSpaces := strings.ReplaceAll(lower, " ", "")
	return strings.TrimRight(noSpaces, "*")
}
