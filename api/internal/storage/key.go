package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// Prefixes allowed by AGENTS.md 5.4.
const (
	PrefixImg    = "img"
	PrefixVideo  = "video"
	PrefixDemos  = "demos"
	PrefixModels = "models"
	PrefixFiles  = "files"
)

var Prefixes = []string{PrefixImg, PrefixVideo, PrefixDemos, PrefixModels, PrefixFiles}

// MaxSize per prefix in bytes.
var MaxSize = map[string]int64{
	PrefixImg:    20 << 20,
	PrefixVideo:  300 << 20,
	PrefixDemos:  500 << 20,
	PrefixModels: 100 << 20,
	PrefixFiles:  100 << 20,
}

// allowedExt maps an extension to the MIME types a client may declare for it.
var allowedExt = map[string][]string{
	// images
	".webp": {"image/webp"},
	".png":  {"image/png"},
	".jpg":  {"image/jpeg"},
	".jpeg": {"image/jpeg"},
	".gif":  {"image/gif"},
	".svg":  {"image/svg+xml"},
	".avif": {"image/avif"},
	// video
	".mp4":  {"video/mp4"},
	".webm": {"video/webm"},
	// fonts
	".woff":  {"font/woff", "application/font-woff"},
	".woff2": {"font/woff2"},
	".ttf":   {"font/ttf", "application/x-font-ttf"},
	".otf":   {"font/otf"},
	// 3d / web builds
	".glb":  {"model/gltf-binary"},
	".gltf": {"model/gltf+json"},
	".bin":  {"application/octet-stream"},
	".wasm": {"application/wasm"},
	".js":   {"text/javascript", "application/javascript"},
	".css":  {"text/css"},
	".html": {"text/html"},
	".json": {"application/json"},
	".data": {"application/octet-stream"},
	".br":   {"application/octet-stream", "application/x-brotli", "application/wasm", "text/javascript", "application/javascript"},
	// misc
	".zip": {"application/zip", "application/x-zip-compressed"},
	".pdf": {"application/pdf"},
}

// ValidateFile checks extension whitelist, MIME consistency and size against the prefix limits.
func ValidateFile(prefix, filename, contentType string, size int64) error {
	if !isPrefix(prefix) {
		return fmt.Errorf("unknown prefix %q; allowed: %s", prefix, strings.Join(Prefixes, ", "))
	}
	ext := strings.ToLower(path.Ext(filename))
	mimes, ok := allowedExt[ext]
	if !ok {
		return fmt.Errorf("extension %q is not allowed", ext)
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if mt, _, err := mime.ParseMediaType(ct); err == nil {
		ct = mt
	}
	if !contains(mimes, ct) {
		return fmt.Errorf("content type %q does not match extension %q", contentType, ext)
	}
	if size <= 0 {
		return errors.New("size must be positive")
	}
	if max := MaxSize[prefix]; size > max {
		return fmt.Errorf("size %d exceeds the %d byte limit for %s/", size, max, prefix)
	}
	return nil
}

// KeyOptions describe how a key is built for a prefix.
type KeyOptions struct {
	Prefix   string
	Filename string
	// For demos/: name and version directories, and the file path inside the build.
	DemoName     string
	DemoVersion  string
	RelativePath string
	Now          time.Time
}

var (
	slugRe    = regexp.MustCompile(`[^a-z0-9._-]+`)
	demoDirRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	keyRe     = regexp.MustCompile(`^(img|video|models|files)/\d{4}/(0[1-9]|1[0-2])/[a-z0-9][a-z0-9._-]*\.[a-z0-9]+$|^demos/[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*/([a-zA-Z0-9._-]+/)*[a-zA-Z0-9._-]+$`)
)

// BuildKey generates the storage key for an upload. Clients never choose keys directly.
//   img/video/models/files: <prefix>/YYYY/MM/<slug>-<rand>.<ext>
//   demos:                  demos/<name>/<version>/<relative path>
func BuildKey(o KeyOptions) (string, error) {
	if !isPrefix(o.Prefix) {
		return "", fmt.Errorf("unknown prefix %q", o.Prefix)
	}
	now := o.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if o.Prefix == PrefixDemos {
		if !demoDirRe.MatchString(o.DemoName) || !demoDirRe.MatchString(o.DemoVersion) {
			return "", errors.New("demos/ uploads need demoName and demoVersion (lowercase letters, digits, . _ -)")
		}
		raw := strings.ReplaceAll(o.RelativePath, "\\", "/")
		if strings.Contains(raw, "..") {
			return "", errors.New("relativePath must not contain ..")
		}
		rel := path.Clean("/" + raw)
		if rel == "/" {
			return "", errors.New("demos/ uploads need a relativePath inside the build directory")
		}
		key := path.Join(PrefixDemos, o.DemoName, o.DemoVersion, strings.TrimPrefix(rel, "/"))
		if !keyRe.MatchString(key) {
			return "", fmt.Errorf("relativePath %q contains unsupported characters", o.RelativePath)
		}
		return key, nil
	}
	ext := strings.ToLower(path.Ext(o.Filename))
	base := strings.TrimSuffix(path.Base(o.Filename), path.Ext(o.Filename))
	base = slugify(base)
	if base == "" {
		base = "file"
	}
	suffix := make([]byte, 3)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}
	key := fmt.Sprintf("%s/%s/%s/%s-%s%s", o.Prefix, now.Format("2006"), now.Format("01"), base, hex.EncodeToString(suffix), ext)
	if !keyRe.MatchString(key) {
		return "", fmt.Errorf("generated key %q is invalid", key)
	}
	return key, nil
}

// ValidKey reports whether key follows the AGENTS.md 5.4 layout (used when serving and deleting).
func ValidKey(key string) bool {
	return keyRe.MatchString(key) && !strings.Contains(key, "..")
}

// PrefixOf returns the top-level prefix of a key.
func PrefixOf(key string) string {
	if i := strings.IndexByte(key, '/'); i > 0 {
		return key[:i]
	}
	return key
}

func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteRune('-')
		}
	}
	out := slugRe.ReplaceAllString(b.String(), "-")
	out = strings.Trim(out, "-.")
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

func isPrefix(p string) bool { return contains(Prefixes, p) }

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
