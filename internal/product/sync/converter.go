package sync

import (
	"strings"

	"storr-api/internal/product"
)

// --- Converters: Shopify -> Storr ---

// StorrProductToShopifyInput converts a Storr product to a Shopify ProductSetInput for upsert.
func StorrProductToShopifyInput(p product.Product) ProductSetInput {
	input := buildBaseProductInput(p)
	input.GiftCard = boolPtrIfTrue(p.GiftCard)
	input.SEO = buildSEOInput(p)
	input.Metafields = storrMetafieldsToInput(p.Metafields)
	input.Variants = buildVariantInputs(p.Variants, p.Options)
	if p.ShopifyID != "" {
		input.ID = p.ShopifyID
	}
	return input
}

func buildBaseProductInput(p product.Product) ProductSetInput {
	return ProductSetInput{
		Handle:          p.Handle,
		Title:           p.Title,
		DescriptionHTML: p.BodyHTML,
		Vendor:          p.Vendor,
		ProductType:     p.ProductType,
		Tags:            p.Tags,
		Status:          storrStatusToShopify(p.Status),
		TemplateSuffix:  p.TemplateSuffix,
		ProductOptions:  buildOptionInputs(p.Options),
	}
}

func boolPtrIfTrue(b bool) *bool {
	if !b {
		return nil
	}
	return &b
}

func buildSEOInput(p product.Product) *seoInput {
	if p.SEOTitle == "" && p.SEODescription == "" {
		return nil
	}
	return &seoInput{Title: p.SEOTitle, Description: p.SEODescription}
}

func storrMetafieldsToInput(mfs []product.Metafield) []metafieldInput {
	if len(mfs) == 0 {
		return nil
	}
	inputs := make([]metafieldInput, len(mfs))
	for i, mf := range mfs {
		inputs[i] = metafieldInput{
			Namespace: mf.Namespace,
			Key:       mf.Key,
			Value:     mf.Value,
			Type:      mf.Type,
		}
	}
	return inputs
}

func storrVariantCostToInput(cost string) *inventoryItemInput {
	if cost == "" {
		return nil
	}
	return &inventoryItemInput{Cost: cost}
}

func shopifyProductToStorr(n productNode) product.Product {
	p := buildBaseStorrProduct(n)
	p.GiftCard = n.IsGiftCard
	p.SEOTitle = n.SEO.Title
	p.SEODescription = n.SEO.Description
	p.Metafields = shopifyMetafieldsToStorr(n.Metafields.Nodes)
	p.Options = convertOptions(n.Options)
	p.Variants = convertVariants(n.Variants)
	p.Images = convertImages(n.Images)
	return p
}

func buildBaseStorrProduct(n productNode) product.Product {
	return product.Product{
		ShopifyID:      n.ID,
		Handle:         n.Handle,
		Title:          n.Title,
		BodyHTML:       n.DescriptionHTML,
		Vendor:         n.Vendor,
		ProductType:    n.ProductType,
		Tags:           n.Tags,
		Status:         product.ShopifyStatusToStorr(n.Status),
		PublishedAt:    product.ParseShopifyTime(n.PublishedAt),
		TemplateSuffix: n.TemplateSuffix,
		SyncStatus:     product.SyncSynced,
	}
}

func shopifyMetafieldsToStorr(nodes []productNodeMF) []product.Metafield {
	if len(nodes) == 0 {
		return nil
	}
	mfs := make([]product.Metafield, len(nodes))
	for i, n := range nodes {
		mfs[i] = product.Metafield{
			Namespace: n.Namespace,
			Key:       n.Key,
			Value:     n.Value,
			Type:      n.Type,
		}
	}
	return mfs
}

func convertOptions(nodes []optionNode) []product.Option {
	opts := make([]product.Option, len(nodes))
	for i, n := range nodes {
		opts[i] = product.Option{Name: n.Name, Position: n.Position, Values: n.Values}
	}
	return opts
}

func convertVariants(conn variantConn) []product.Variant {
	variants := make([]product.Variant, len(conn.Edges))
	for i, e := range conn.Edges {
		variants[i] = shopifyVariantToStorr(e.Node)
	}
	return variants
}

func shopifyVariantToStorr(n variantNode) product.Variant {
	v := buildBaseVariant(n)
	if n.CompareAtPrice != nil {
		v.CompareAtPrice = *n.CompareAtPrice
	}
	if n.InventoryItem.UnitCost != nil {
		v.CostPerItem = n.InventoryItem.UnitCost.Amount
	}
	v = assignOptionValues(v, n.SelectedOptions)
	if n.Image != nil {
		v.ImageID = n.Image.ID
	}
	return v
}

func buildBaseVariant(n variantNode) product.Variant {
	return product.Variant{
		ShopifyVariantID: n.ID,
		Title:            n.Title,
		SKU:              n.SKU,
		Barcode:          n.Barcode,
		Price:            n.Price,
		Position:         n.Position,
		InventoryQty:     n.InventoryQuantity,
		InventoryPolicy:  strings.ToLower(n.InventoryPolicy),
		Taxable:          n.Taxable,
		TaxCode:          n.TaxCode,
		RequiresShipping: n.RequiresShipping,
		Weight:           n.Weight,
		WeightUnit:       shopifyWeightUnitToStorr(n.WeightUnit),
		Grams:            weightToGrams(n.Weight, n.WeightUnit),
	}
}

func assignOptionValues(v product.Variant, opts []selectedOption) product.Variant {
	for i, opt := range opts {
		switch i {
		case 0:
			v.Option1 = opt.Value
		case 1:
			v.Option2 = opt.Value
		case 2:
			v.Option3 = opt.Value
		}
	}
	return v
}

func convertImages(conn imageConn) []product.Image {
	images := make([]product.Image, len(conn.Edges))
	for i, e := range conn.Edges {
		images[i] = product.Image{
			ImageID:  e.Node.ID,
			URL:      e.Node.URL,
			Name:     e.Node.AltText,
			AltText:  e.Node.AltText,
			Position: i + 1,
			Width:    e.Node.Width,
			Height:   e.Node.Height,
		}
	}
	return images
}

// --- Converters: Storr -> Shopify ---

func buildOptionInputs(opts []product.Option) []OptionSetInput {
	inputs := make([]OptionSetInput, len(opts))
	for i, o := range opts {
		values := make([]OptionValue, len(o.Values))
		for j, v := range o.Values {
			values[j] = OptionValue{Name: v}
		}
		inputs[i] = OptionSetInput{Name: o.Name, Values: values}
	}
	return inputs
}

func buildVariantInputs(variants []product.Variant, options []product.Option) []VariantSetInput {
	inputs := make([]VariantSetInput, len(variants))
	for i, v := range variants {
		inputs[i] = storrVariantToInput(v, options)
	}
	return inputs
}

func storrVariantToInput(v product.Variant, options []product.Option) VariantSetInput {
	input := VariantSetInput{
		SKU:              v.SKU,
		Barcode:          v.Barcode,
		Price:            v.Price,
		CompareAtPrice:   v.CompareAtPrice,
		Position:         v.Position,
		InventoryPolicy:  storrPolicyToShopify(v.InventoryPolicy),
		Taxable:          v.Taxable,
		TaxCode:          v.TaxCode,
		RequiresShipping: v.RequiresShipping,
		Weight:           v.Weight,
		WeightUnit:       storrWeightUnitToShopify(v.WeightUnit),
		InventoryItem:    storrVariantCostToInput(v.CostPerItem),
		OptionValues:     buildVariantOptionRefs(v, options),
	}
	if v.ShopifyVariantID != "" {
		input.ID = v.ShopifyVariantID
	}
	return input
}

func buildVariantOptionRefs(v product.Variant, options []product.Option) []OptionValueRef {
	optVals := [3]string{v.Option1, v.Option2, v.Option3}
	var refs []OptionValueRef
	for i, opt := range options {
		if i < 3 && optVals[i] != "" {
			refs = append(refs, OptionValueRef{OptionName: opt.Name, Name: optVals[i]})
		}
	}
	return refs
}

// --- Enum Converters ---

func storrStatusToShopify(s product.ProductStatus) string {
	switch s {
	case product.StatusActive:
		return "ACTIVE"
	case product.StatusArchived:
		return "ARCHIVED"
	default:
		return "DRAFT"
	}
}

func shopifyWeightUnitToStorr(s string) string {
	switch strings.ToUpper(s) {
	case "POUNDS":
		return "lb"
	case "OUNCES":
		return "oz"
	case "KILOGRAMS":
		return "kg"
	case "GRAMS":
		return "g"
	default:
		return product.DefaultWeightUnit
	}
}

func storrWeightUnitToShopify(s string) string {
	switch strings.ToLower(s) {
	case "lb":
		return "POUNDS"
	case "oz":
		return "OUNCES"
	case "kg":
		return "KILOGRAMS"
	case "g":
		return "GRAMS"
	default:
		return "POUNDS"
	}
}

func storrPolicyToShopify(s string) string {
	if strings.EqualFold(s, "continue") {
		return "CONTINUE"
	}
	return "DENY"
}

func weightToGrams(weight float64, unit string) int {
	switch strings.ToUpper(unit) {
	case "POUNDS":
		return int(weight * GramsPerPound)
	case "OUNCES":
		return int(weight * GramsPerOunce)
	case "KILOGRAMS":
		return int(weight * GramsPerKilogram)
	case "GRAMS":
		return int(weight)
	default:
		return int(weight * GramsPerPound)
	}
}

