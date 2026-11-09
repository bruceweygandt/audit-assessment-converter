package main

import (
	"encoding/json"
	"strings"
)

// RichTextNode represents a Plate editor paragraph node.
type RichTextNode struct {
	Type     string          `json:"type"`
	Children []RichTextChild `json:"children"`
}

// RichTextChild can be a text node or a link node.
// Text node: {"text": "content"} or {"bold": true, "text": "content"}
// Link node: {"type": "a", "url": "...", "children": [{"text": "..."}]}
type RichTextChild struct {
	// Text node fields
	Text string
	Bold bool

	// Link node fields
	IsLink   bool
	URL      string
	LinkText string
}

func (c RichTextChild) MarshalJSON() ([]byte, error) {
	if c.IsLink {
		obj := map[string]interface{}{
			"type": "a",
			"url":  c.URL,
			"children": []map[string]interface{}{
				{"text": c.LinkText},
			},
		}
		return json.Marshal(obj)
	}
	obj := map[string]interface{}{
		"text": c.Text,
	}
	if c.Bold {
		obj["bold"] = true
	}
	return json.Marshal(obj)
}

func (c *RichTextChild) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if typeRaw, ok := raw["type"]; ok {
		var t string
		if err := json.Unmarshal(typeRaw, &t); err == nil && t == "a" {
			c.IsLink = true
			if urlRaw, ok := raw["url"]; ok {
				json.Unmarshal(urlRaw, &c.URL)
			}
			if childrenRaw, ok := raw["children"]; ok {
				var children []struct {
					Text string `json:"text"`
				}
				json.Unmarshal(childrenRaw, &children)
				if len(children) > 0 {
					c.LinkText = children[0].Text
				}
			}
			return nil
		}
	}

	if textRaw, ok := raw["text"]; ok {
		json.Unmarshal(textRaw, &c.Text)
	}
	if boldRaw, ok := raw["bold"]; ok {
		json.Unmarshal(boldRaw, &c.Bold)
	}
	return nil
}

// TextToParagraphs converts plain text into Plate rich text paragraph nodes.
// Splits on newlines; each line becomes its own paragraph node.
// Empty text returns nil.
func TextToParagraphs(text string) []RichTextNode {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var nodes []RichTextNode
	for _, line := range lines {
		nodes = append(nodes, RichTextNode{
			Type: "p",
			Children: []RichTextChild{
				{Text: strings.TrimSpace(line)},
			},
		})
	}
	return nodes
}

// BoldHeaderParagraph creates a single paragraph with bold text.
func BoldHeaderParagraph(text string) RichTextNode {
	return RichTextNode{
		Type: "p",
		Children: []RichTextChild{
			{Text: text, Bold: true},
		},
	}
}

// EmptyParagraph returns a single empty paragraph node.
func EmptyParagraph() []RichTextNode {
	return []RichTextNode{
		{Type: "p", Children: []RichTextChild{{Text: ""}}},
	}
}

// ColumnValue pairs a column name with its cell value.
type ColumnValue struct {
	Name  string
	Value string
}

// BuildSectionedRichText builds a field from one or more mapped columns: each
// column with non-empty content becomes a bold heading (the column's name)
// followed by its text, in mapping order. Used for result and plan.
func BuildSectionedRichText(columns []ColumnValue) []RichTextNode {
	var nodes []RichTextNode
	for _, col := range columns {
		value := strings.TrimSpace(col.Value)
		if value == "" {
			continue
		}
		nodes = append(nodes, BoldHeaderParagraph(col.Name))
		nodes = append(nodes, TextToParagraphs(value)...)
	}
	if len(nodes) == 0 {
		return EmptyParagraph()
	}
	return nodes
}

// BuildFieldRichText creates rich text for a single-column field (plan or remediation).
// Returns nil if the value is empty.
func BuildFieldRichText(value string) []RichTextNode {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return TextToParagraphs(value)
}
