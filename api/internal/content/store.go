package content

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Store is a content directory with its validator.
type Store struct {
	Root      string // absolute path of content/
	Validator *Validator
}

// NewStore opens content/ and loads content/schema/.
func NewStore(root string) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("content dir %s does not exist", abs)
	}
	v, err := LoadValidator(filepath.Join(abs, "schema"))
	if err != nil {
		return nil, err
	}
	return &Store{Root: abs, Validator: v}, nil
}

// Entry is a summary row of a post or work.
type Entry struct {
	Slug        string         `json:"slug"`
	Path        string         `json:"path"` // relative to the repository root, forward slashes
	Title       string         `json:"title"`
	Date        string         `json:"date,omitempty"`
	Description string         `json:"description,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Categories  []string       `json:"categories,omitempty"`
	Draft       bool           `json:"draft"`
	Status      string         `json:"status,omitempty"`
	Frontmatter map[string]any `json:"-"`
}

// ErrExists is returned when creating a slug that already exists.
var ErrExists = errors.New("already exists")

// ErrNotFound is returned for unknown slugs.
var ErrNotFound = errors.New("not found")

/* ---------- posts ---------- */

func (s *Store) postsDir() string { return filepath.Join(s.Root, "posts") }

// FindPost returns the file path for slug in any year directory.
func (s *Store) FindPost(slug string) (string, error) {
	if !SlugRe.MatchString(slug) {
		return "", fmt.Errorf("invalid slug %q", slug)
	}
	var found string
	err := filepath.WalkDir(s.postsDir(), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if d.Name() == slug+".mdx" {
			found = p
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", ErrNotFound
	}
	return found, nil
}

// ListPosts returns every post, newest first.
func (s *Store) ListPosts() ([]Entry, error) {
	var out []Entry
	err := filepath.WalkDir(s.postsDir(), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".mdx") {
			return err
		}
		doc, err := s.read(p)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		e := entryFrom(doc, strings.TrimSuffix(d.Name(), ".mdx"), s.rel(p))
		out = append(out, e)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date > out[j].Date
		}
		return out[i].Slug < out[j].Slug
	})
	return out, nil
}

// GetPost reads one post.
func (s *Store) GetPost(slug string) (*Document, string, error) {
	p, err := s.FindPost(slug)
	if err != nil {
		return nil, "", err
	}
	doc, err := s.read(p)
	return doc, s.rel(p), err
}

// CreatePost validates and writes content/posts/<year>/<slug>.mdx. Empty slug derives one
// from the title. Returns the repository-relative path.
func (s *Store) CreatePost(frontmatter map[string]any, body, slug string) (string, error) {
	fm := cloneMap(frontmatter)
	if _, ok := fm["date"]; !ok {
		fm["date"] = time.Now().Format("2006-01-02")
	}
	if err := s.Validator.Validate("posts", fm); err != nil {
		return "", err
	}
	title, _ := fm["title"].(string)
	if slug == "" {
		slug = Slugify(title)
	}
	if !SlugRe.MatchString(slug) {
		return "", fmt.Errorf("cannot derive a valid slug from %q; pass slug explicitly", title)
	}
	if _, err := s.FindPost(slug); err == nil {
		return "", fmt.Errorf("post %q %w", slug, ErrExists)
	}
	date, _ := fm["date"].(string)
	year := date[:4]
	p := filepath.Join(s.postsDir(), year, slug+".mdx")
	doc := &Document{Frontmatter: fm, Body: body}
	return s.rel(p), s.write(p, doc, PostKeyOrder)
}

// UpdatePost merges patch into the front matter (unset removes keys) and optionally replaces the body.
func (s *Store) UpdatePost(slug string, patch map[string]any, unset []string, body *string) (string, error) {
	p, err := s.FindPost(slug)
	if err != nil {
		return "", err
	}
	doc, err := s.read(p)
	if err != nil {
		return "", err
	}
	for _, k := range unset {
		delete(doc.Frontmatter, k)
	}
	for k, v := range patch {
		doc.Frontmatter[k] = Normalize(v)
	}
	if err := s.Validator.Validate("posts", doc.Frontmatter); err != nil {
		return "", err
	}
	if body != nil {
		doc.Body = *body
	}
	return s.rel(p), s.write(p, doc, PostKeyOrder)
}

/* ---------- works ---------- */

func (s *Store) worksDir() string { return filepath.Join(s.Root, "works") }

func (s *Store) workPath(slug string) (string, error) {
	if !SlugRe.MatchString(slug) {
		return "", fmt.Errorf("invalid slug %q", slug)
	}
	return filepath.Join(s.worksDir(), slug+".mdx"), nil
}

// ListWorks returns every work sorted by slug.
func (s *Store) ListWorks() ([]Entry, error) {
	entries, err := os.ReadDir(s.worksDir())
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, d := range entries {
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".mdx") {
			continue
		}
		p := filepath.Join(s.worksDir(), d.Name())
		doc, err := s.read(p)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		out = append(out, entryFrom(doc, strings.TrimSuffix(d.Name(), ".mdx"), s.rel(p)))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// GetWork reads one work.
func (s *Store) GetWork(slug string) (*Document, string, error) {
	p, err := s.workPath(slug)
	if err != nil {
		return nil, "", err
	}
	if _, err := os.Stat(p); err != nil {
		return nil, "", ErrNotFound
	}
	doc, err := s.read(p)
	return doc, s.rel(p), err
}

// CreateWork validates and writes content/works/<slug>.mdx.
func (s *Store) CreateWork(frontmatter map[string]any, body, slug string) (string, error) {
	fm := cloneMap(frontmatter)
	if err := s.Validator.Validate("works", fm); err != nil {
		return "", err
	}
	title, _ := fm["title"].(string)
	if slug == "" {
		slug = Slugify(title)
	}
	p, err := s.workPath(slug)
	if err != nil {
		return "", fmt.Errorf("cannot derive a valid slug from %q; pass slug explicitly", title)
	}
	if _, err := os.Stat(p); err == nil {
		return "", fmt.Errorf("work %q %w", slug, ErrExists)
	}
	return s.rel(p), s.write(p, &Document{Frontmatter: fm, Body: body}, WorkKeyOrder)
}

// UpdateWork merges patch into the front matter and optionally replaces the body.
func (s *Store) UpdateWork(slug string, patch map[string]any, unset []string, body *string) (string, error) {
	doc, _, err := s.GetWork(slug)
	if err != nil {
		return "", err
	}
	p, _ := s.workPath(slug)
	for _, k := range unset {
		delete(doc.Frontmatter, k)
	}
	for k, v := range patch {
		doc.Frontmatter[k] = Normalize(v)
	}
	if err := s.Validator.Validate("works", doc.Frontmatter); err != nil {
		return "", err
	}
	if body != nil {
		doc.Body = *body
	}
	return s.rel(p), s.write(p, doc, WorkKeyOrder)
}

/* ---------- helpers ---------- */

func (s *Store) read(p string) (*Document, error) {
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return Parse(string(raw))
}

func (s *Store) write(p string, doc *Document, order []string) error {
	text, err := doc.Serialize(order)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(text), 0o644)
}

// rel returns the path relative to the repository root (parent of content/), forward slashes.
func (s *Store) rel(p string) string {
	r, err := filepath.Rel(filepath.Dir(s.Root), p)
	if err != nil {
		return filepath.ToSlash(p)
	}
	return filepath.ToSlash(r)
}

func entryFrom(doc *Document, slug, path string) Entry {
	fm := doc.Frontmatter
	e := Entry{Slug: slug, Path: path, Frontmatter: fm}
	e.Title, _ = fm["title"].(string)
	e.Date, _ = fm["date"].(string)
	e.Description, _ = fm["description"].(string)
	if e.Description == "" {
		e.Description, _ = fm["summary"].(string)
	}
	e.Tags = stringSlice(fm["tags"])
	e.Categories = stringSlice(fm["categories"])
	e.Draft, _ = fm["draft"].(bool)
	e.Status, _ = fm["status"].(string)
	return e
}

func stringSlice(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, x := range list {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = Normalize(v)
	}
	return out
}
