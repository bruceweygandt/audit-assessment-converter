package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCSV_Normal(t *testing.T) {
	content := "Name,Age,City\nAlice,30,NYC\nBob,25,LA\n"
	path := writeTempCSV(t, content)

	data, err := ParseCSV(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Headers) != 3 {
		t.Fatalf("expected 3 headers, got %d", len(data.Headers))
	}
	if data.Headers[0] != "Name" || data.Headers[1] != "Age" || data.Headers[2] != "City" {
		t.Errorf("unexpected headers: %v", data.Headers)
	}
	if len(data.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(data.Rows))
	}
	if data.Rows[0][0] != "Alice" {
		t.Errorf("expected 'Alice', got %q", data.Rows[0][0])
	}
}

func TestParseCSV_WithBOM(t *testing.T) {
	content := "\xEF\xBB\xBFName,Value\nTest,123\n"
	path := writeTempCSV(t, content)

	data, err := ParseCSV(path)
	if err != nil {
		t.Fatal(err)
	}
	if data.Headers[0] != "Name" {
		t.Errorf("expected 'Name' (BOM stripped), got %q", data.Headers[0])
	}
}

func TestParseCSV_WithNewlinesInFields(t *testing.T) {
	content := "ID,Description\n1,\"Line one\nLine two\"\n2,Simple\n"
	path := writeTempCSV(t, content)

	data, err := ParseCSV(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(data.Rows))
	}
	if data.Rows[0][1] != "Line one\nLine two" {
		t.Errorf("expected multiline field, got %q", data.Rows[0][1])
	}
}

func TestParseCSV_EmptyFile(t *testing.T) {
	path := writeTempCSV(t, "Header\n")
	_, err := ParseCSV(path)
	if err == nil {
		t.Error("expected error for CSV with no data rows")
	}
}

func TestParseCSV_ExampleFile(t *testing.T) {
	path := "../../example_files/20x Phase Two Report - Paramify - 01.26.2026 - Human-Readable (1).csv"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("example CSV file not found")
	}

	data, err := ParseCSV(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Headers) != 7 {
		t.Errorf("expected 7 headers, got %d", len(data.Headers))
	}
	if len(data.Rows) < 200 {
		t.Errorf("expected at least 200 rows, got %d", len(data.Rows))
	}
	if data.Headers[0] != "Indicator ID" {
		t.Errorf("expected first header 'Indicator ID', got %q", data.Headers[0])
	}
}

func writeTempCSV(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.csv")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
