package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Local stores files under <root>/ and serves them itself. It simulates the COS flow: Presign
// hands out a one-time PUT URL on this server, and the public URL points at /assets/<key>.
type Local struct {
	root       string
	publicBase string // e.g. http://localhost:8080/assets
	uploadBase string // e.g. http://localhost:8080/_local/upload

	mu     sync.Mutex
	tokens map[string]localToken
}

type localToken struct {
	key         string
	contentType string
	size        int64
	expires     time.Time
}

const localTokenTTL = 15 * time.Minute

// NewLocal creates the adapter. publicBase and uploadBase are absolute URLs without trailing slash.
func NewLocal(root, publicBase, uploadBase string) (*Local, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Local{
		root:       root,
		publicBase: strings.TrimRight(publicBase, "/"),
		uploadBase: strings.TrimRight(uploadBase, "/"),
		tokens:     map[string]localToken{},
	}, nil
}

func (l *Local) Presign(_ context.Context, key, contentType string, size int64) (*PresignedUpload, error) {
	if !ValidKey(key) {
		return nil, fmt.Errorf("invalid key %q", key)
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(raw)
	exp := time.Now().Add(localTokenTTL)
	l.mu.Lock()
	for t, v := range l.tokens {
		if time.Now().After(v.expires) {
			delete(l.tokens, t)
		}
	}
	l.tokens[token] = localToken{key: key, contentType: contentType, size: size, expires: exp}
	l.mu.Unlock()
	return &PresignedUpload{
		URL:       l.uploadBase + "/" + token,
		Method:    http.MethodPut,
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: exp,
	}, nil
}

func (l *Local) PublicURL(key string) string { return l.publicBase + "/" + key }

func (l *Local) Delete(_ context.Context, key string) error {
	p, err := l.pathFor(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (l *Local) List(_ context.Context, prefix string) ([]ObjectInfo, error) {
	var out []ObjectInfo
	err := filepath.WalkDir(l.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(l.root, p)
		key := filepath.ToSlash(rel)
		if !strings.HasPrefix(key, prefix) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		out = append(out, ObjectInfo{Key: key, Size: info.Size(), ModTime: info.ModTime(), ContentType: ContentTypeFor(key)})
		return nil
	})
	return out, err
}

func (l *Local) Stat(_ context.Context, key string) (*ObjectInfo, error) {
	p, err := l.pathFor(key)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ObjectInfo{Key: key, Size: info.Size(), ModTime: info.ModTime(), ContentType: ContentTypeFor(key)}, nil
}

// UploadHandler receives the PUT issued by Presign: PUT /_local/upload/{token}.
func (l *Local) UploadHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.PathValue("token")
		l.mu.Lock()
		t, ok := l.tokens[token]
		if ok {
			delete(l.tokens, token)
		}
		l.mu.Unlock()
		if !ok || time.Now().After(t.expires) {
			http.Error(w, "unknown or expired upload token", http.StatusForbidden)
			return
		}
		if ct := r.Header.Get("Content-Type"); ct != "" && !sameMediaType(ct, t.contentType) {
			http.Error(w, "content type does not match the presigned upload", http.StatusBadRequest)
			return
		}
		if r.ContentLength > 0 && r.ContentLength != t.size {
			http.Error(w, "content length does not match the presigned upload", http.StatusBadRequest)
			return
		}
		p, err := l.pathFor(t.key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmp := p + ".part"
		f, err := os.Create(tmp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		n, err := io.Copy(f, io.LimitReader(r.Body, t.size+1))
		f.Close()
		if err != nil || n != t.size {
			os.Remove(tmp)
			http.Error(w, "upload size mismatch", http.StatusBadRequest)
			return
		}
		if err := os.Rename(tmp, p); err != nil {
			os.Remove(tmp)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

// ServeHandler serves GET /assets/{key...} from disk with the headers a CDN would need:
// Brotli pre-compressed files, COOP/COEP for demos/, permissive CORS/CORP for cross-origin embedding.
func (l *Local) ServeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		if !ValidKey(key) {
			http.NotFound(w, r)
			return
		}
		p, err := l.pathFor(key)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(p)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		h := w.Header()
		SetAssetHeaders(h, key)
		h.Set("Cache-Control", "public, max-age=3600")
		http.ServeContent(w, r, path.Base(key), info.ModTime(), f)
	})
}

// SetAssetHeaders applies the content-type, encoding and isolation headers for an asset key.
// Nginx/COS must reproduce the same rules in production (see deploy/).
func SetAssetHeaders(h http.Header, key string) {
	h.Set("Content-Type", ContentTypeFor(key))
	if strings.HasSuffix(key, ".br") {
		h.Set("Content-Encoding", "br")
		h.Add("Vary", "Accept-Encoding")
	}
	if strings.HasPrefix(key, PrefixDemos+"/") {
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Embedder-Policy", "require-corp")
	}
	h.Set("Cross-Origin-Resource-Policy", "cross-origin")
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("X-Content-Type-Options", "nosniff")
}

// ContentTypeFor infers the content type from the key, looking through a trailing .br.
func ContentTypeFor(key string) string {
	name := key
	if strings.HasSuffix(name, ".br") {
		name = strings.TrimSuffix(name, ".br")
	}
	switch ext := strings.ToLower(path.Ext(name)); ext {
	case ".wasm":
		return "application/wasm"
	case ".js":
		return "text/javascript"
	case ".data", ".bin", ".unityweb":
		return "application/octet-stream"
	case ".webp":
		return "image/webp"
	case ".glb":
		return "model/gltf-binary"
	case ".gltf":
		return "model/gltf+json"
	case ".woff2":
		return "font/woff2"
	case "":
		return "application/octet-stream"
	default:
		if ct := mime.TypeByExtension(ext); ct != "" {
			if strings.HasPrefix(ct, "text/") && !strings.Contains(ct, "charset") {
				return ct + "; charset=utf-8"
			}
			return ct
		}
		return "application/octet-stream"
	}
}

func (l *Local) pathFor(key string) (string, error) {
	if !ValidKey(key) {
		return "", fmt.Errorf("invalid key %q", key)
	}
	p := filepath.Join(l.root, filepath.FromSlash(key))
	if rel, err := filepath.Rel(l.root, p); err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("key %q escapes the storage root", key)
	}
	return p, nil
}

func sameMediaType(a, b string) bool {
	ma, _, _ := mime.ParseMediaType(a)
	mb, _, _ := mime.ParseMediaType(b)
	return strings.EqualFold(ma, mb)
}

// SetBases changes the public and upload base URLs after construction (tests bind to a random port).
func (l *Local) SetBases(publicBase, uploadBase string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.publicBase = strings.TrimRight(publicBase, "/")
	l.uploadBase = strings.TrimRight(uploadBase, "/")
}

// Put writes an object straight to disk.
func (l *Local) Put(_ context.Context, key, _ string, r io.Reader, size int64) error {
	p, err := l.pathFor(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, r)
	f.Close()
	if err != nil || (size > 0 && n != size) {
		os.Remove(tmp)
		if err == nil {
			err = fmt.Errorf("wrote %d bytes, expected %d", n, size)
		}
		return err
	}
	return os.Rename(tmp, p)
}
