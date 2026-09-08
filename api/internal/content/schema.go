package content

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var printer = message.NewPrinter(language.English)

// Validator checks front matter against content/schema/<collection>.schema.json, the JSON
// Schemas exported from the site's zod definitions (`pnpm -C site schema`).
type Validator struct {
	schemas map[string]*jsonschema.Schema
}

// Collections that have a schema file.
var Collections = []string{"posts", "works", "pages", "friends"}

// LoadValidator compiles every schema under dir.
func LoadValidator(dir string) (*Validator, error) {
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	v := &Validator{schemas: map[string]*jsonschema.Schema{}}
	for _, name := range Collections {
		path := filepath.Join(dir, name+".schema.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("schema %s: %w (run `pnpm -C site schema`)", name, err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("schema %s: %w", name, err)
		}
		url := "content/schema/" + name + ".schema.json"
		if err := c.AddResource(url, doc); err != nil {
			return nil, err
		}
		sch, err := c.Compile(url)
		if err != nil {
			return nil, fmt.Errorf("compile schema %s: %w", name, err)
		}
		v.schemas[name] = sch
	}
	return v, nil
}

// Validate returns a readable error listing every violation, or nil.
func (v *Validator) Validate(collection string, frontmatter map[string]any) error {
	sch, ok := v.schemas[collection]
	if !ok {
		return fmt.Errorf("no schema for collection %q", collection)
	}
	// Round-trip through JSON so numbers and nested maps have the shapes the validator expects.
	raw, err := json.Marshal(frontmatter)
	if err != nil {
		return err
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	if err := sch.Validate(value); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			return &FieldError{Collection: collection, Problems: flatten(ve)}
		}
		return err
	}
	return nil
}

// FieldError lists schema violations with their field paths.
type FieldError struct {
	Collection string
	Problems   []string
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("%s front matter is invalid: %s", e.Collection, strings.Join(e.Problems, "; "))
}

func flatten(ve *jsonschema.ValidationError) []string {
	var out []string
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			loc := "/" + strings.Join(e.InstanceLocation, "/")
			if loc == "/" {
				loc = "(root)"
			}
			out = append(out, fmt.Sprintf("%s: %s", loc, e.ErrorKind.LocalizedString(printer)))
			return
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	return out
}
