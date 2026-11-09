package main

import (
	"encoding/json"
	"testing"
)

func TestTextToParagraphs_SingleLine(t *testing.T) {
	nodes := TextToParagraphs("Hello world")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Type != "p" {
		t.Errorf("expected type 'p', got %q", nodes[0].Type)
	}
	if nodes[0].Children[0].Text != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", nodes[0].Children[0].Text)
	}
}

func TestTextToParagraphs_MultiLine(t *testing.T) {
	nodes := TextToParagraphs("Line one\nLine two\nLine three")
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}
	expected := []string{"Line one", "Line two", "Line three"}
	for i, e := range expected {
		if nodes[i].Children[0].Text != e {
			t.Errorf("node %d: expected %q, got %q", i, e, nodes[i].Children[0].Text)
		}
	}
}

func TestTextToParagraphs_Empty(t *testing.T) {
	nodes := TextToParagraphs("")
	if nodes != nil {
		t.Errorf("expected nil for empty text, got %v", nodes)
	}
}

func TestTextToParagraphs_WhitespaceOnly(t *testing.T) {
	nodes := TextToParagraphs("   \n  \n  ")
	if nodes != nil {
		t.Errorf("expected nil for whitespace-only text, got %v", nodes)
	}
}

func TestTextToParagraphs_CarriageReturn(t *testing.T) {
	nodes := TextToParagraphs("Line one\r\nLine two")
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Children[0].Text != "Line one" {
		t.Errorf("expected 'Line one', got %q", nodes[0].Children[0].Text)
	}
	if nodes[1].Children[0].Text != "Line two" {
		t.Errorf("expected 'Line two', got %q", nodes[1].Children[0].Text)
	}
}

func TestBoldHeaderParagraph(t *testing.T) {
	node := BoldHeaderParagraph("My Header")
	if node.Type != "p" {
		t.Errorf("expected type 'p', got %q", node.Type)
	}
	if !node.Children[0].Bold {
		t.Error("expected bold to be true")
	}
	if node.Children[0].Text != "My Header" {
		t.Errorf("expected 'My Header', got %q", node.Children[0].Text)
	}
}

func TestBuildSectionedRichText_MultipleColumns(t *testing.T) {
	cols := []ColumnValue{
		{Name: "Automation", Value: "Auto text"},
		{Name: "Overview", Value: "Overview text"},
	}
	nodes := BuildSectionedRichText(cols)
	// Should produce: bold "Automation", text "Auto text", bold "Overview", text "Overview text"
	if len(nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(nodes))
	}
	if !nodes[0].Children[0].Bold || nodes[0].Children[0].Text != "Automation" {
		t.Errorf("expected bold 'Automation' header")
	}
	if nodes[1].Children[0].Text != "Auto text" {
		t.Errorf("expected 'Auto text', got %q", nodes[1].Children[0].Text)
	}
	if !nodes[2].Children[0].Bold || nodes[2].Children[0].Text != "Overview" {
		t.Errorf("expected bold 'Overview' header")
	}
	if nodes[3].Children[0].Text != "Overview text" {
		t.Errorf("expected 'Overview text', got %q", nodes[3].Children[0].Text)
	}
}

func TestBuildSectionedRichText_SkipEmpty(t *testing.T) {
	cols := []ColumnValue{
		{Name: "Filled", Value: "Some content"},
		{Name: "Empty", Value: ""},
		{Name: "Also Filled", Value: "More content"},
	}
	nodes := BuildSectionedRichText(cols)
	// Empty column should be skipped: bold "Filled", text, bold "Also Filled", text
	if len(nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(nodes))
	}
	if nodes[2].Children[0].Text != "Also Filled" {
		t.Errorf("expected 'Also Filled', got %q", nodes[2].Children[0].Text)
	}
}

func TestBuildSectionedRichText_AllEmpty(t *testing.T) {
	cols := []ColumnValue{
		{Name: "A", Value: ""},
		{Name: "B", Value: "  "},
	}
	nodes := BuildSectionedRichText(cols)
	// Should return default empty paragraph
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node (empty paragraph), got %d", len(nodes))
	}
	if nodes[0].Children[0].Text != "" {
		t.Errorf("expected empty text, got %q", nodes[0].Children[0].Text)
	}
}

func TestBuildSectionedRichText_MultiLineContent(t *testing.T) {
	cols := []ColumnValue{
		{Name: "Notes", Value: "Line 1\nLine 2"},
	}
	nodes := BuildSectionedRichText(cols)
	// bold "Notes", text "Line 1", text "Line 2"
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}
}

func TestBuildFieldRichText_NonEmpty(t *testing.T) {
	nodes := BuildFieldRichText("Some plan text")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Children[0].Text != "Some plan text" {
		t.Errorf("unexpected text: %q", nodes[0].Children[0].Text)
	}
}

func TestBuildFieldRichText_Empty(t *testing.T) {
	nodes := BuildFieldRichText("")
	if nodes != nil {
		t.Errorf("expected nil for empty text, got %v", nodes)
	}
}

func TestRichTextChild_MarshalText(t *testing.T) {
	child := RichTextChild{Text: "hello"}
	data, err := json.Marshal(child)
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"text":"hello"}`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, string(data))
	}
}

func TestRichTextChild_MarshalBold(t *testing.T) {
	child := RichTextChild{Text: "bold text", Bold: true}
	data, err := json.Marshal(child)
	if err != nil {
		t.Fatal(err)
	}
	// Should contain both "bold":true and "text"
	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if m["bold"] != true {
		t.Errorf("expected bold to be true")
	}
	if m["text"] != "bold text" {
		t.Errorf("expected text to be 'bold text'")
	}
}

func TestRichTextChild_MarshalLink(t *testing.T) {
	child := RichTextChild{
		IsLink:   true,
		URL:      "https://example.com",
		LinkText: "Click here",
	}
	data, err := json.Marshal(child)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	json.Unmarshal(data, &m)
	if m["type"] != "a" {
		t.Errorf("expected type 'a', got %v", m["type"])
	}
	if m["url"] != "https://example.com" {
		t.Errorf("expected url 'https://example.com'")
	}
	children := m["children"].([]interface{})
	firstChild := children[0].(map[string]interface{})
	if firstChild["text"] != "Click here" {
		t.Errorf("expected link text 'Click here'")
	}
}

func TestRichTextChild_UnmarshalText(t *testing.T) {
	data := []byte(`{"text":"hello"}`)
	var child RichTextChild
	if err := json.Unmarshal(data, &child); err != nil {
		t.Fatal(err)
	}
	if child.Text != "hello" {
		t.Errorf("expected 'hello', got %q", child.Text)
	}
	if child.Bold {
		t.Error("expected bold to be false")
	}
	if child.IsLink {
		t.Error("expected IsLink to be false")
	}
}

func TestRichTextChild_UnmarshalBold(t *testing.T) {
	data := []byte(`{"bold":true,"text":"bold text"}`)
	var child RichTextChild
	if err := json.Unmarshal(data, &child); err != nil {
		t.Fatal(err)
	}
	if child.Text != "bold text" {
		t.Errorf("expected 'bold text', got %q", child.Text)
	}
	if !child.Bold {
		t.Error("expected bold to be true")
	}
}

func TestRichTextChild_UnmarshalLink(t *testing.T) {
	data := []byte(`{"type":"a","url":"https://example.com","children":[{"text":"link"}]}`)
	var child RichTextChild
	if err := json.Unmarshal(data, &child); err != nil {
		t.Fatal(err)
	}
	if !child.IsLink {
		t.Error("expected IsLink to be true")
	}
	if child.URL != "https://example.com" {
		t.Errorf("expected url 'https://example.com', got %q", child.URL)
	}
	if child.LinkText != "link" {
		t.Errorf("expected link text 'link', got %q", child.LinkText)
	}
}

func TestRichTextNode_FullMarshal(t *testing.T) {
	node := RichTextNode{
		Type: "p",
		Children: []RichTextChild{
			{Text: "Before "},
			{IsLink: true, URL: "https://example.com", LinkText: "click"},
			{Text: "."},
		},
	}
	data, err := json.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}

	// Verify it round-trips
	var parsed RichTextNode
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(parsed.Children))
	}
	if parsed.Children[0].Text != "Before " {
		t.Errorf("expected 'Before ', got %q", parsed.Children[0].Text)
	}
	if !parsed.Children[1].IsLink {
		t.Error("expected second child to be a link")
	}
	if parsed.Children[2].Text != "." {
		t.Errorf("expected '.', got %q", parsed.Children[2].Text)
	}
}
