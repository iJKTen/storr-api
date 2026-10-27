package importexport

import (
	"strconv"
	"strings"

	"storr-api/internal/product"
)

const (
	valColHandle         = "Handle"
	valColTitle          = "Title"
	valColVariantPrice   = "Variant Price"
	valColCompareAtPrice = "Variant Compare At Price"
	valColCostPerItem    = "Cost Per Item"

	valMsgHandleRequired   = "Handle is required"
	valMsgTitleRequired    = "Title is required"
	valMsgPriceRequired    = "Price is required"
	valMsgPriceInvalid     = "Price must be a valid number"
	valMsgCompareAtInvalid = "Compare at price must be a valid number"
	valMsgCostInvalid      = "Cost per item must be a valid number"
)

// parsedRow represents a single row from an import file with mapped field values.
type parsedRow struct {
	rowNumber int
	fields    map[string]string
}

// rowGroup holds rows that share the same handle and will become one product.
type rowGroup struct {
	rows []parsedRow
}

// processRows groups parsed rows by handle, converts to products, and validates.
// Products with validation errors are excluded from the returned slice.
func processRows(rows []parsedRow) ([]product.Product, []product.ValidationError) {
	groups := groupByHandle(rows)
	var products []product.Product
	var allErrors []product.ValidationError
	for _, g := range groups {
		p, errs := convertGroup(g)
		allErrors = append(allErrors, errs...)
		if len(errs) == 0 {
			products = append(products, p)
		}
	}
	return products, allErrors
}

func groupByHandle(rows []parsedRow) []rowGroup {
	orderMap := make(map[string]int)
	var groups []rowGroup
	for _, row := range rows {
		handle := row.fields[fieldHandle]
		if idx, exists := orderMap[handle]; exists {
			groups[idx].rows = append(groups[idx].rows, row)
		} else {
			orderMap[handle] = len(groups)
			groups = append(groups, rowGroup{rows: []parsedRow{row}})
		}
	}
	return groups
}

func convertGroup(group rowGroup) (product.Product, []product.ValidationError) {
	firstRow := group.rows[0]
	p := extractProductFields(firstRow.fields)
	p.Variants, p.Images = extractVariantsAndImages(group.rows)
	names := optionNames{
		Name1: firstRow.fields[fieldOption1Name],
		Name2: firstRow.fields[fieldOption2Name],
		Name3: firstRow.fields[fieldOption3Name],
	}
	p.Options = buildOptions(p.Variants, names)
	errs := validateProduct(p, firstRow.rowNumber)
	return p, errs
}

func isVariantRow(fields map[string]string) bool {
	return fields[fieldVariantPrice] != "" ||
		fields[fieldOption1Value] != "" ||
		fields[fieldOption2Value] != "" ||
		fields[fieldOption3Value] != "" ||
		fields[fieldVariantSKU] != ""
}

func extractVariantsAndImages(rows []parsedRow) ([]product.Variant, []product.Image) {
	var variants []product.Variant
	var images []product.Image
	for i, row := range rows {
		if isVariantRow(row.fields) {
			variants = append(variants, extractVariant(row.fields, i+1))
		}
		if row.fields[fieldImageSrc] != "" {
			images = append(images, extractImage(row.fields))
		}
	}
	setImagePositions(images)
	return variants, images
}

func extractProductFields(fields map[string]string) product.Product {
	return product.Product{
		Handle:         fields[fieldHandle],
		Title:          fields[fieldTitle],
		BodyHTML:       fields[fieldBodyHTML],
		Vendor:         fields[fieldVendor],
		ProductType:    fields[fieldProductType],
		Tags:           product.SplitTags(fields[fieldTags]),
		Status:         product.ProductStatus(fields[fieldStatus]),
		Published:      parseBoolField(fields[fieldPublished]),
		PublishedScope: fields[fieldPublishedScope],
		TemplateSuffix: fields[fieldTemplateSuffix],
		SEOTitle:       fields[fieldSEOTitle],
		SEODescription: fields[fieldSEODesc],
		GiftCard:       parseBoolField(fields[fieldGiftCard]),
		Metafields:     extractMetafields(fields),
	}
}

func extractVariant(fields map[string]string, position int) product.Variant {
	v := extractVariantBase(fields)
	v.Position = position
	v.Option2 = fields[fieldOption2Value]
	v.Option3 = fields[fieldOption3Value]
	v.CostPerItem = fields[fieldCostPerItem]
	v.Grams = parseIntField(fields[fieldVariantGrams])
	v.Weight = parseFloatField(fields[fieldVariantWeight])
	v.WeightUnit = fields[fieldVariantWeightUnit]
	v.InventoryQty = parseIntField(fields[fieldInventoryQty])
	return v
}

func extractVariantBase(fields map[string]string) product.Variant {
	return product.Variant{
		SKU:                 fields[fieldVariantSKU],
		Barcode:             fields[fieldVariantBarcode],
		Price:               fields[fieldVariantPrice],
		CompareAtPrice:      fields[fieldCompareAtPrice],
		Option1:             fields[fieldOption1Value],
		InventoryPolicy:     fields[fieldInventoryPolicy],
		InventoryManagement: fields[fieldInventoryManagement],
		FulfillmentService:  fields[fieldFulfillmentService],
		Taxable:             parseBoolField(fields[fieldVariantTaxable]),
		TaxCode:             fields[fieldVariantTaxCode],
		RequiresShipping:    parseBoolField(fields[fieldRequiresShipping]),
	}
}

func extractImage(fields map[string]string) product.Image {
	return product.Image{
		URL:     fields[fieldImageSrc],
		AltText: fields[fieldImageAlt],
	}
}

func extractMetafields(fields map[string]string) []product.Metafield {
	var mfs []product.Metafield
	for key, val := range fields {
		if !strings.HasPrefix(key, metafieldPrefix) {
			continue
		}
		if val == "" {
			continue
		}
		nsKey := key[len(metafieldPrefix):]
		ns, k := splitMetafieldNSKey(nsKey)
		if ns == "" {
			continue
		}
		mfs = append(mfs, buildMetafield(ns, k, val))
	}
	return mfs
}

func splitMetafieldNSKey(nsKey string) (string, string) {
	idx := strings.Index(nsKey, ".")
	if idx < 0 {
		return "", ""
	}
	return nsKey[:idx], nsKey[idx+1:]
}

func buildMetafield(ns, key, val string) product.Metafield {
	return product.Metafield{
		Namespace: ns,
		Key:       key,
		Value:     val,
		Type:      product.DefaultMetafieldType,
	}
}

func setImagePositions(images []product.Image) {
	for i := range images {
		images[i].Position = i + 1
	}
}

type optionNames struct {
	Name1 string
	Name2 string
	Name3 string
}

func buildOptions(variants []product.Variant, names optionNames) []product.Option {
	var opts []product.Option
	if o := buildOptionAxis(variants, 1, names.Name1); o != nil {
		opts = append(opts, *o)
	}
	if o := buildOptionAxis(variants, 2, names.Name2); o != nil {
		opts = append(opts, *o)
	}
	if o := buildOptionAxis(variants, 3, names.Name3); o != nil {
		opts = append(opts, *o)
	}
	return opts
}

func buildOptionAxis(variants []product.Variant, pos int, name string) *product.Option {
	values := collectOptionValues(variants, pos)
	if len(values) == 0 {
		return nil
	}
	if name == "" {
		name = defaultOptionNameFor(pos)
	}
	return &product.Option{Name: name, Position: pos, Values: values}
}

func collectOptionValues(variants []product.Variant, pos int) []string {
	seen := make(map[string]bool)
	var values []string
	for _, v := range variants {
		val := optionValueAt(v, pos)
		if val != "" && !seen[val] {
			seen[val] = true
			values = append(values, val)
		}
	}
	return values
}

func optionValueAt(v product.Variant, pos int) string {
	switch pos {
	case 1:
		return v.Option1
	case 2:
		return v.Option2
	case 3:
		return v.Option3
	default:
		return ""
	}
}

func defaultOptionNameFor(pos int) string {
	switch pos {
	case 1:
		return defaultOptionName
	case 2:
		return defaultOption2Name
	case 3:
		return defaultOption3Name
	default:
		return defaultOptionName
	}
}

// --- Validation ---

func validateProduct(p product.Product, startRow int) []product.ValidationError {
	var errs []product.ValidationError
	errs = append(errs, validateRequiredFields(p, startRow)...)
	for i, v := range p.Variants {
		errs = append(errs, validateVariant(v, startRow+i)...)
	}
	return errs
}

func validateRequiredFields(p product.Product, row int) []product.ValidationError {
	var errs []product.ValidationError
	if p.Handle == "" {
		errs = append(errs, product.ValidationError{Row: row, Column: valColHandle, Message: valMsgHandleRequired})
	}
	if p.Title == "" {
		errs = append(errs, product.ValidationError{Row: row, Column: valColTitle, Message: valMsgTitleRequired})
	}
	return errs
}

func validateVariant(v product.Variant, row int) []product.ValidationError {
	var errs []product.ValidationError
	if v.Price == "" {
		errs = append(errs, product.ValidationError{Row: row, Column: valColVariantPrice, Message: valMsgPriceRequired})
		return errs
	}
	if !isValidPrice(v.Price) {
		errs = append(errs, product.ValidationError{Row: row, Column: valColVariantPrice, Message: valMsgPriceInvalid})
	}
	if v.CompareAtPrice != "" && !isValidPrice(v.CompareAtPrice) {
		errs = append(errs, product.ValidationError{Row: row, Column: valColCompareAtPrice, Message: valMsgCompareAtInvalid})
	}
	if v.CostPerItem != "" && !isValidPrice(v.CostPerItem) {
		errs = append(errs, product.ValidationError{Row: row, Column: valColCostPerItem, Message: valMsgCostInvalid})
	}
	return errs
}

func isValidPrice(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// --- Shared Parsing Helpers ---

func buildParsedRow(rowNum int, record, headers []string, mappings map[string]string) parsedRow {
	fields := make(map[string]string, len(mappings))
	for i, header := range headers {
		fieldName, ok := mappings[header]
		if !ok || i >= len(record) {
			continue
		}
		fields[fieldName] = strings.TrimSpace(record[i])
	}
	return parsedRow{rowNumber: rowNum, fields: fields}
}

func parseIntField(s string) int {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int(f)
}

func parseFloatField(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func parseBoolField(s string) bool {
	return strings.EqualFold(s, "true")
}
