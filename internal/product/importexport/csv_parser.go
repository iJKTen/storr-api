package importexport

import (
	"encoding/csv"
	"fmt"
	"io"

	"storr-api/internal/product"
)

// ParseCSV reads a CSV file, maps columns to product fields, and returns
// parsed products with any validation errors. If mappings is nil, column
// headers are auto-detected using the mapping registry.
func ParseCSV(r io.Reader, mappings map[string]string) ([]product.Product, []product.ValidationError) {
	reader := csv.NewReader(r)
	headers, err := reader.Read()
	if err != nil {
		return nil, []product.ValidationError{{Row: 1, Message: "Failed to read CSV headers"}}
	}
	if len(mappings) == 0 {
		mappings = DetectMappings(headers)
	}
	rows, errs := readCSVRows(reader, headers, mappings)
	if len(errs) > 0 {
		return nil, errs
	}
	return processRows(rows)
}

func readCSVRows(reader *csv.Reader, headers []string, mappings map[string]string) ([]parsedRow, []product.ValidationError) {
	var rows []parsedRow
	rowNum := 2
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			msg := fmt.Sprintf("Failed to read row: %v", err)
			return nil, []product.ValidationError{{Row: rowNum, Message: msg}}
		}
		rows = append(rows, buildParsedRow(rowNum, record, headers, mappings))
		rowNum++
	}
	return rows, nil
}
