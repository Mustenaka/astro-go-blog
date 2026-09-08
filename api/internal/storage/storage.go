// Package storage abstracts where uploaded assets live: the local disk in development,
// Tencent Cloud COS in production. Files never pass through the Go process; the server only
// issues pre-signed upload targets and records metadata.
package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

// PresignedUpload tells a client where and how to PUT a file.
type PresignedUpload struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

// ObjectInfo is what the backend can learn about a stored object.
type ObjectInfo struct {
	Key         string    `json:"key"`
	Size        int64     `json:"size"`
	ContentType string    `json:"contentType"`
	ModTime     time.Time `json:"modTime"`
}

// Storage is implemented by the local adapter and the COS adapter.
type Storage interface {
	// Presign issues an upload target for key. The client must send exactly contentType and size.
	Presign(ctx context.Context, key, contentType string, size int64) (*PresignedUpload, error)
	// PublicURL returns the URL the public site should use for key.
	PublicURL(key string) string
	// Delete removes the object; deleting a missing object is not an error.
	Delete(ctx context.Context, key string) error
	// List returns objects under prefix (no pagination; used for reconciliation and tests).
	List(ctx context.Context, prefix string) ([]ObjectInfo, error)
	// Stat returns metadata for key or ErrNotFound.
	Stat(ctx context.Context, key string) (*ObjectInfo, error)
	// Put stores an object directly. Used by local tooling (the MCP server); browser uploads
	// never go through this path.
	Put(ctx context.Context, key, contentType string, r io.Reader, size int64) error
}

// ErrNotFound is returned by Stat for missing objects.
var ErrNotFound = errors.New("object not found")
