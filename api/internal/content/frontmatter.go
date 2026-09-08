// Package content reads and writes the MDX files under content/. It is the only way any
// backend tool changes content: files in, files out, validated against the JSON Schemas
// exported by the site.
package content

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Document is a parsed MDX file: ordered front matter plus the body.
type Document struct {
	Frontmatter map[string]any
	// Order of the keys as they appeared in the file (used to keep diffs small on rewrite).
	Order []string
	// Style of top-level sequences (flow `[a, b]` vs block) to preserve on rewrite.
	styles map[string]yaml.Style
	Body   string
}

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// Parse splits `---` front matter from the body and decodes the YAML.
func Parse(src string) (*Document, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	if !strings.HasPrefix(src, "---\n") {
		return nil, errors.New("file does not start with a --- front matter block")
	}
	rest := src[4:]
	end := strings.Index(rest, "\n---\n")
	var yamlText, body string
	switch {
	case end >= 0:
		yamlText, body = rest[:end], rest[end+5:]
	case strings.HasSuffix(rest, "\n---"):
		yamlText, body = strings.TrimSuffix(rest, "\n---"), ""
	default:
		return nil, errors.New("front matter block is not closed with ---")
	}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(yamlText), &root); err != nil {
		return nil, fmt.Errorf("front matter YAML: %w", err)
	}
	doc := &Document{Frontmatter: map[string]any{}, styles: map[string]yaml.Style{}, Body: strings.TrimPrefix(body, "\n")}
	if root.Kind == 0 || len(root.Content) == 0 {
		return doc, nil
	}
	mapping := root.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return nil, errors.New("front matter must be a YAML mapping")
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keyNode, valNode := mapping.Content[i], mapping.Content[i+1]
		key := keyNode.Value
		var value any
		if err := valNode.Decode(&value); err != nil {
			return nil, fmt.Errorf("front matter field %q: %w", key, err)
		}
		doc.Frontmatter[key] = Normalize(value)
		doc.Order = append(doc.Order, key)
		if valNode.Kind == yaml.SequenceNode {
			doc.styles[key] = valNode.Style
		}
	}
	return doc, nil
}

// Normalize turns YAML-decoded values into JSON-compatible ones: time.Time becomes
// "YYYY-MM-DD" (or RFC 3339 when it has a time part), nested maps get string keys.
func Normalize(v any) any {
	switch t := v.(type) {
	case time.Time:
		if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
			return t.Format("2006-01-02")
		}
		return t.Format(time.RFC3339)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = Normalize(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fmt.Sprint(k)] = Normalize(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = Normalize(val)
		}
		return out
	default:
		return v
	}
}

// dateKeys are emitted as bare timestamps (`date: 2026-09-01`), not quoted strings.
var dateKeys = map[string]bool{"date": true, "updated": true}

// Serialize writes the document back. Known keys come first in `order`, then the remaining
// keys in the order they were read, then any new keys alphabetically.
func (d *Document) Serialize(order []string) (string, error) {
	seen := map[string]bool{}
	var keys []string
	add := func(k string) {
		if _, ok := d.Frontmatter[k]; ok && !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for _, k := range order {
		add(k)
	}
	for _, k := range d.Order {
		add(k)
	}
	var rest []string
	for k := range d.Frontmatter {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sortStrings(rest)
	for _, k := range rest {
		add(k)
	}

	mapping := &yaml.Node{Kind: yaml.MappingNode}
	for _, k := range keys {
		valNode, err := d.valueNode(k, d.Frontmatter[k])
		if err != nil {
			return "", err
		}
		mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k}, valNode)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{mapping}}); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	body := strings.TrimRight(d.Body, "\n")
	out := "---\n" + buf.String() + "---\n"
	if body != "" {
		out += "\n" + body + "\n"
	}
	return out, nil
}

func (d *Document) valueNode(key string, v any) (*yaml.Node, error) {
	if s, ok := v.(string); ok && dateKeys[key] && dateRe.MatchString(s) {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!timestamp", Value: s}, nil
	}
	n := &yaml.Node{}
	if err := n.Encode(v); err != nil {
		return nil, fmt.Errorf("field %q: %w", key, err)
	}
	if n.Kind == yaml.SequenceNode {
		if style, ok := d.styles[key]; ok {
			n.Style = style
		} else if isScalarSequence(n) && len(n.Content) <= 8 {
			n.Style = yaml.FlowStyle
		}
	}
	return n, nil
}

func isScalarSequence(n *yaml.Node) bool {
	for _, c := range n.Content {
		if c.Kind != yaml.ScalarNode {
			return false
		}
	}
	return true
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// PostKeyOrder and WorkKeyOrder mirror the tables in AGENTS.md section 5.
var (
	PostKeyOrder = []string{"title", "description", "date", "updated", "tags", "categories", "draft", "cover", "math", "lang", "series", "legacyUrls", "toc"}
	WorkKeyOrder = []string{"title", "summary", "period", "role", "stack", "links", "cover", "gallery", "demo", "featured", "order", "status"}
)
