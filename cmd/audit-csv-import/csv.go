package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
)

// CSVData holds parsed CSV headers and rows.
type CSVData struct {
	Headers []string
	Rows    [][]string
}

// ParseCSV reads and parses a CSV file, handling UTF-8 BOM and multiline fields.
func ParseCSV(path string) (*CSVData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening CSV: %w", err)
	}
	defer f.Close()

	reader := bufio.NewReader(f)

	// Handle UTF-8 BOM
	bom, err := reader.Peek(3)
	if err == nil && len(bom) >= 3 && bom[0] == 0xEF && bom[1] == 0xBB && bom[2] == 0xBF {
		reader.Discard(3)
	}

	csvReader := csv.NewReader(reader)
	csvReader.LazyQuotes = true
	csvReader.FieldsPerRecord = -1

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV must have at least a header row and one data row")
	}

	return &CSVData{
		Headers: records[0],
		Rows:    records[1:],
	}, nil
}
