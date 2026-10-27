package importexport

import (
	"bytes"
	"encoding/csv"
	"fmt"

	"storr-api/internal/product"

	"github.com/xuri/excelize/v2"
)

const (
	productsSheetName    = "Products"
	columnGuideSheetName = "Column Guide"
	templateHeaderColor  = "#D6E4F0"
	guideHeaderColor     = "#E2EFDA"
	requiredMarker       = " *"
	productColumnWidth   = 22.0
	guideColNameWidth    = 25.0
	guideColReqWidth     = 12.0
	guideColDescWidth    = 55.0
	guideColExWidth      = 30.0
	guideColAccWidth     = 30.0
)

// GenerateTemplate creates a downloadable Excel workbook with a "Products"
// sheet (headers + example rows) and a "Column Guide" reference sheet.
func GenerateTemplate() (*bytes.Buffer, error) {
	f := excelize.NewFile()
	defer f.Close()
	if err := buildProductsSheet(f); err != nil {
		return nil, fmt.Errorf("products sheet: %w", err)
	}
	if err := buildColumnGuideSheet(f); err != nil {
		return nil, fmt.Errorf("column guide: %w", err)
	}
	f.DeleteSheet("Sheet1")
	idx, _ := f.GetSheetIndex(productsSheetName)
	f.SetActiveSheet(idx)
	return writeWorkbook(f)
}

// GenerateCSVTemplate creates a CSV file with headers and example rows.
func GenerateCSVTemplate() ([]byte, error) {
	buf := new(bytes.Buffer)
	w := csv.NewWriter(buf)
	if err := w.Write(templateHeaders()); err != nil {
		return nil, fmt.Errorf("write csv headers: %w", err)
	}
	for _, row := range exampleRows() {
		if err := w.Write(row); err != nil {
			return nil, fmt.Errorf("write csv row: %w", err)
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func templateHeaders() []string {
	mappings := MappingRegistry()
	headers := make([]string, len(mappings))
	for i, m := range mappings {
		headers[i] = m.DisplayName
	}
	return headers
}

func writeWorkbook(f *excelize.File) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	if err := f.Write(buf); err != nil {
		return nil, fmt.Errorf("write workbook: %w", err)
	}
	return buf, nil
}

// --- Products Sheet ---

func buildProductsSheet(f *excelize.File) error {
	if _, err := f.NewSheet(productsSheetName); err != nil {
		return err
	}
	if err := writeProductHeaders(f); err != nil {
		return err
	}
	if err := writeProductExampleRows(f); err != nil {
		return err
	}
	if err := setProductColumnWidths(f); err != nil {
		return err
	}
	return addStatusValidation(f)
}

func writeProductHeaders(f *excelize.File) error {
	mappings := MappingRegistry()
	style, err := createHeaderStyle(f, templateHeaderColor)
	if err != nil {
		return err
	}
	for i, m := range mappings {
		cell, err := cellRef(1, i+1)
		if err != nil {
			return err
		}
		ci := cellInput{Sheet: productsSheetName, Cell: cell, Value: headerLabel(m), Style: style}
		if err := writeStyledCell(f, ci); err != nil {
			return err
		}
	}
	return freezeHeaderRow(f, productsSheetName)
}

func headerLabel(m ColumnMapping) string {
	if m.Required {
		return m.DisplayName + requiredMarker
	}
	return m.DisplayName
}

func writeProductExampleRows(f *excelize.File) error {
	for rowIdx, row := range exampleRows() {
		for colIdx, val := range row {
			cell, err := cellRef(rowIdx+2, colIdx+1)
			if err != nil {
				return err
			}
			if err := f.SetCellValue(productsSheetName, cell, val); err != nil {
				return err
			}
		}
	}
	return nil
}

func setProductColumnWidths(f *excelize.File) error {
	colCount := len(MappingRegistry())
	lastCol, err := excelize.ColumnNumberToName(colCount)
	if err != nil {
		return fmt.Errorf("column name: %w", err)
	}
	return f.SetColWidth(productsSheetName, "A", lastCol, productColumnWidth)
}

func addStatusValidation(f *excelize.File) error {
	col := statusColumnIndex()
	startCell, err := cellRef(2, col)
	if err != nil {
		return err
	}
	endCell, err := cellRef(100, col)
	if err != nil {
		return err
	}
	dv := excelize.NewDataValidation(true)
	dv.Sqref = startCell + ":" + endCell
	dv.SetDropList([]string{
		string(product.StatusActive),
		string(product.StatusDraft),
		string(product.StatusArchived),
	})
	return f.AddDataValidation(productsSheetName, dv)
}

func statusColumnIndex() int {
	for i, m := range MappingRegistry() {
		if m.FieldName == fieldStatus {
			return i + 1
		}
	}
	return 7
}

// --- Example Data ---

func exampleRows() [][]string {
	return [][]string{
		exampleRow1(),
		exampleRow2(),
		exampleRow3(),
	}
}

func exampleRow1() []string {
	return append(exampleProductA(), exampleVariantA1()...)
}

func exampleProductA() []string {
	return []string{
		"example-t-shirt",
		"Example T-Shirt",
		"<p>Soft cotton tee in classic black</p>",
		"Acme Co",
		"Shirts",
		"summer, cotton, sale",
		"active",
		"true",
		"global",
		"",
		"Example T-Shirt | Store",
		"Shop our cotton t-shirt...",
		"false",
	}
}

func exampleVariantA1() []string {
	return []string{
		"Size",
		"Small",
		"",
		"",
		"",
		"",
		"TSHIRT-S-BLK",
		"123456789012",
		"29.99",
		"39.99",
		"15.00",
		"100",
		"deny",
		"shopify",
		"manual",
		"true",
		"",
		"true",
		"0.5",
		"lb",
		"227",
		"https://cdn.example.com/tshirt-front.png",
		"Black t-shirt front view",
		"black.front",
	}
}

func exampleRow2() []string {
	p := make([]string, len(productMappings()))
	p[0] = "example-t-shirt"
	return append(p, exampleVariantA2()...)
}

func exampleVariantA2() []string {
	return []string{
		"",
		"Medium",
		"",
		"",
		"",
		"",
		"TSHIRT-M-BLK",
		"123456789013",
		"29.99",
		"39.99",
		"15.00",
		"150",
		"deny",
		"shopify",
		"manual",
		"true",
		"",
		"true",
		"0.5",
		"lb",
		"227",
		"",
		"",
		"",
	}
}

func exampleRow3() []string {
	return append(exampleProductB(), exampleVariantB1()...)
}

func exampleProductB() []string {
	return []string{
		"classic-hoodie",
		"Classic Hoodie",
		"<p>Warm fleece hoodie for cold days</p>",
		"Acme Co",
		"Hoodies",
		"winter, fleece",
		"draft",
		"false",
		"global",
		"",
		"",
		"",
		"false",
	}
}

func exampleVariantB1() []string {
	return []string{
		"Size",
		"One Size",
		"",
		"",
		"",
		"",
		"HOODIE-OS-GRY",
		"987654321098",
		"59.99",
		"",
		"25.00",
		"50",
		"deny",
		"shopify",
		"manual",
		"true",
		"",
		"true",
		"1.2",
		"lb",
		"544",
		"https://cdn.example.com/hoodie.png",
		"Grey hoodie front view",
		"",
	}
}

// --- Column Guide Sheet ---

type columnGuideEntry struct {
	column      string
	required    string
	description string
	example     string
	accepted    string
}

func buildColumnGuideSheet(f *excelize.File) error {
	if _, err := f.NewSheet(columnGuideSheetName); err != nil {
		return err
	}
	if err := writeGuideHeaders(f); err != nil {
		return err
	}
	if err := writeGuideEntries(f); err != nil {
		return err
	}
	if err := setGuideColumnWidths(f); err != nil {
		return err
	}
	return writeGuideTips(f)
}

func writeGuideHeaders(f *excelize.File) error {
	headers := []string{"Column Name", "Required?", "Description", "Example", "Accepted Values"}
	style, err := createHeaderStyle(f, guideHeaderColor)
	if err != nil {
		return err
	}
	for i, h := range headers {
		cell, err := cellRef(1, i+1)
		if err != nil {
			return err
		}
		ci := cellInput{Sheet: columnGuideSheetName, Cell: cell, Value: h, Style: style}
		if err := writeStyledCell(f, ci); err != nil {
			return err
		}
	}
	return freezeHeaderRow(f, columnGuideSheetName)
}

func writeGuideEntries(f *excelize.File) error {
	entries := columnGuideEntries()
	for i, e := range entries {
		row := i + 2
		vals := []string{e.column, e.required, e.description, e.example, e.accepted}
		for j, val := range vals {
			cell, err := cellRef(row, j+1)
			if err != nil {
				return err
			}
			if err := f.SetCellValue(columnGuideSheetName, cell, val); err != nil {
				return err
			}
		}
	}
	return nil
}

func setGuideColumnWidths(f *excelize.File) error {
	sheet := columnGuideSheetName
	if err := f.SetColWidth(sheet, "A", "A", guideColNameWidth); err != nil {
		return err
	}
	if err := f.SetColWidth(sheet, "B", "B", guideColReqWidth); err != nil {
		return err
	}
	if err := f.SetColWidth(sheet, "C", "C", guideColDescWidth); err != nil {
		return err
	}
	if err := f.SetColWidth(sheet, "D", "D", guideColExWidth); err != nil {
		return err
	}
	return f.SetColWidth(sheet, "E", "E", guideColAccWidth)
}

func columnGuideEntries() []columnGuideEntry {
	return append(productGuideEntries(), variantGuideEntries()...)
}

func productGuideEntries() []columnGuideEntry {
	return []columnGuideEntry{
		{"Handle", "Yes", "URL-friendly identifier. Rows with the same handle are grouped into one product.", "example-t-shirt", "Lowercase letters, numbers, hyphens"},
		{"Title", "Yes", "The product name displayed to customers.", "Example T-Shirt", "Any text"},
		{"Body (HTML)", "No", "Product description. Supports HTML formatting.", "<p>Soft cotton tee</p>", "HTML or plain text"},
		{"Vendor", "No", "The product brand or manufacturer.", "Acme Co", "Any text"},
		{"Type", "No", "Product category for organizing your catalog.", "Shirts", "Any text"},
		{"Tags", "No", "Comma-separated keywords for search and filtering.", "summer, cotton, sale", "Comma-separated text"},
		{"Status", "No", "Publication status. Defaults to draft if omitted.", "active", "active, draft, archived"},
		{"Published", "No", "Whether the product is visible on the online store.", "true", "true, false"},
		{"Published Scope", "No", "Where the product is visible. Defaults to global.", "global", "global, web"},
		{"Template Suffix", "No", "Custom Liquid template suffix for this product.", "", "Template file suffix"},
		{"SEO Title", "No", "Custom page title for search engines. Leave blank to use product title.", "Summer T-Shirt | Store", "Any text"},
		{"SEO Description", "No", "Custom meta description for search engines.", "Shop our cotton t-shirt...", "Any text (max 320 chars)"},
		{"Gift Card", "No", "Whether this product is a gift card.", "false", "true, false"},
	}
}

func variantGuideEntries() []columnGuideEntry {
	return append(variantFieldGuide(), imageFieldGuide()...)
}

func variantFieldGuide() []columnGuideEntry {
	return []columnGuideEntry{
		{"Option1 Name", "No", "Name of the first option axis (e.g. Size, Color). Only fill on the first row.", "Size", "Any text"},
		{"Option1 Value", "No", "Value for the first option axis (e.g. size or color).", "Small", "Any text"},
		{"Option2 Name", "No", "Name of the second option axis.", "Color", "Any text"},
		{"Option2 Value", "No", "Value for the second option.", "Red", "Any text"},
		{"Option3 Name", "No", "Name of the third option axis.", "Material", "Any text"},
		{"Option3 Value", "No", "Value for the third option.", "Cotton", "Any text"},
		{"Variant SKU", "No", "Stock keeping unit identifier for inventory tracking.", "TSHIRT-S-BLK", "Any text"},
		{"Variant Barcode", "No", "UPC, EAN, or ISBN barcode for the variant.", "123456789012", "Numeric barcode"},
		{"Variant Price", "Yes", "Selling price of the variant.", "29.99", "Decimal number"},
		{"Variant Compare At Price", "No", "Original price shown as strikethrough during sales.", "39.99", "Decimal number"},
		{"Cost Per Item", "No", "Cost you paid for this variant (for profit reports).", "15.00", "Decimal number"},
		{"Variant Inventory Qty", "No", "Number of units currently in stock.", "100", "Whole number"},
		{"Variant Inventory Policy", "No", "Behavior when inventory reaches zero. Defaults to deny.", "deny", "deny, continue"},
		{"Variant Inventory Tracker", "No", "Inventory tracking provider. Leave blank to disable.", "shopify", "shopify or blank"},
		{"Variant Fulfillment Service", "No", "How orders are fulfilled. Defaults to manual.", "manual", "manual or service name"},
		{"Variant Taxable", "No", "Whether tax is charged on this variant.", "true", "true, false"},
		{"Variant Tax Code", "No", "Tax code for Shopify Tax (Shopify Plus only).", "", "Tax code string"},
		{"Variant Requires Shipping", "No", "Whether this variant needs physical shipping.", "true", "true, false"},
		{"Variant Weight", "No", "Numeric weight of the variant.", "0.5", "Decimal number"},
		{"Variant Weight Unit", "No", "Unit of measurement for variant weight.", "lb", "lb, kg, oz, g"},
		{"Variant Grams", "No", "Weight in grams (alternative to weight + unit).", "200", "Whole number"},
	}
}

func imageFieldGuide() []columnGuideEntry {
	return []columnGuideEntry{
		{"Image Src", "No", "URL of the product image.", "https://cdn.example.com/tshirt.png", "Valid URL"},
		{"Image Alt Text", "No", "Accessibility description for the image.", "Black t-shirt front view", "Any text"},
		{"Image Name", "No", "Optional label for variant image filtering. Format: {optionValue}.{view} (e.g. black.front). Used by themes to show variant-specific images.", "black.front", "Any text, e.g. blue.back"},
	}
}

func writeGuideTips(f *excelize.File) error {
	startRow := len(columnGuideEntries()) + 4
	return writeTipRows(f, startRow, guideTips())
}

func guideTips() []string {
	return []string{
		"Tips:",
		"- Rows with the same Handle are grouped into one product with multiple variants.",
		"- Only fill product-level fields (Title, Body, Vendor, etc.) on the first row of each group.",
		"- Leave product-level fields blank on subsequent variant rows.",
		"- Each variant must have a unique Option1 Value within its product.",
		"- Only fill Option1 Name on the first row of each product group.",
		"- Variant Price is required for every variant row.",
		"- Set Published to true to make the product visible on your online store.",
		"- Columns marked with * in the Products sheet are required.",
		"- Option2/Option3 columns follow the same pattern as Option1.",
		"- Metafield columns use the format: Name (product.metafields.namespace.key)",
		"- SEO fields override the auto-generated search engine listing.",
	}
}

func writeTipRows(f *excelize.File, startRow int, tips []string) error {
	boldStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	if err != nil {
		return err
	}
	for i, tip := range tips {
		cell, err := cellRef(startRow+i, 1)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(columnGuideSheetName, cell, tip); err != nil {
			return err
		}
		if i == 0 {
			if err := f.SetCellStyle(columnGuideSheetName, cell, cell, boldStyle); err != nil {
				return err
			}
		}
	}
	return nil
}

// --- Shared Helpers ---

func createHeaderStyle(f *excelize.File, color string) (int, error) {
	return f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{color}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
}

func freezeHeaderRow(f *excelize.File, sheet string) error {
	return f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})
}

func cellRef(row, col int) (string, error) {
	colName, err := excelize.ColumnNumberToName(col)
	if err != nil {
		return "", fmt.Errorf("column number to name: %w", err)
	}
	cell, err := excelize.JoinCellName(colName, row)
	if err != nil {
		return "", fmt.Errorf("join cell name: %w", err)
	}
	return cell, nil
}

type cellInput struct {
	Sheet string
	Cell  string
	Value string
	Style int
}

func writeStyledCell(f *excelize.File, ci cellInput) error {
	if err := f.SetCellValue(ci.Sheet, ci.Cell, ci.Value); err != nil {
		return err
	}
	return f.SetCellStyle(ci.Sheet, ci.Cell, ci.Cell, ci.Style)
}
