package importexport

import (
	"io"
	"strings"

	"storr-api/internal/product"

	"github.com/xuri/excelize/v2"
)

// ParseXLSX reads an Excel file, maps columns to product fields, and returns
// parsed products with any validation errors. If mappings is nil, column
// headers are auto-detected using the mapping registry.
func ParseXLSX(r io.Reader, mappings map[string]string) ([]product.Product, []product.ValidationError) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, []product.ValidationError{{Row: 0, Message: "Failed to open Excel file"}}
	}
	defer f.Close()
	sheet := resolveSheetName(f)
	if sheet == "" {
		return nil, []product.ValidationError{{Row: 0, Message: "No sheets found in workbook"}}
	}
	rows, errs := readXLSXRows(f, sheet, mappings)
	if len(errs) > 0 {
		return nil, errs
	}
	return processRows(rows)
}

func resolveSheetName(f *excelize.File) string {
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return ""
	}
	for _, name := range sheets {
		if strings.EqualFold(name, productsSheetName) {
			return name
		}
	}
	return sheets[0]
}

func readXLSXRows(f *excelize.File, sheet string, mappings map[string]string) ([]parsedRow, []product.ValidationError) {
	allRows, err := f.GetRows(sheet)
	if err != nil || len(allRows) == 0 {
		return nil, []product.ValidationError{{Row: 0, Message: "No data found in spreadsheet"}}
	}
	headers := allRows[0]
	if len(mappings) == 0 {
		mappings = DetectMappings(headers)
	}
	return convertXLSXRows(allRows[1:], headers, mappings), nil
}

func convertXLSXRows(dataRows [][]string, headers []string, mappings map[string]string) []parsedRow {
	rows := make([]parsedRow, 0, len(dataRows))
	for i, record := range dataRows {
		if isEmptyRow(record) {
			continue
		}
		rows = append(rows, buildParsedRow(i+2, record, headers, mappings))
	}
	return rows
}

func isEmptyRow(record []string) bool {
	for _, cell := range record {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}
