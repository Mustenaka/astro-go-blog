package storage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

// COSConfig configures the Tencent Cloud COS adapter.
type COSConfig struct {
	Bucket     string // e.g. blog-1250000000
	Region     string // e.g. ap-guangzhou
	SecretID   string
	SecretKey  string
	PublicBase string // CDN origin + /assets, e.g. https://cdn.example.com/assets
}

// COS issues pre-signed PUT URLs for Tencent Cloud COS and reads object metadata.
// The public URL is the CDN base plus the key; browsers never talk to the bucket for reads.
type COS struct {
	client     *cos.Client
	cfg        COSConfig
	publicBase string
}

const cosPresignTTL = 15 * time.Minute

// NewCOS creates the adapter. No network call is made here.
func NewCOS(cfg COSConfig) (*COS, error) {
	if cfg.Bucket == "" || cfg.Region == "" || cfg.SecretID == "" || cfg.SecretKey == "" || cfg.PublicBase == "" {
		return nil, errors.New("cos: bucket, region, secret id/key and public base are required")
	}
	bucketURL, err := cos.NewBucketURL(cfg.Bucket, cfg.Region, true)
	if err != nil {
		return nil, fmt.Errorf("cos: %w", err)
	}
	client := cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Timeout: 30 * time.Second,
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.SecretID,
			SecretKey: cfg.SecretKey,
		},
	})
	return &COS{client: client, cfg: cfg, publicBase: strings.TrimRight(cfg.PublicBase, "/")}, nil
}

func (c *COS) Presign(ctx context.Context, key, contentType string, size int64) (*PresignedUpload, error) {
	if !ValidKey(key) {
		return nil, fmt.Errorf("invalid key %q", key)
	}
	headers := http.Header{}
	headers.Set("Content-Type", contentType)
	opt := &cos.PresignedURLOptions{Header: &headers}
	u, err := c.client.Object.GetPresignedURL(ctx, http.MethodPut, key, c.cfg.SecretID, c.cfg.SecretKey, cosPresignTTL, opt)
	if err != nil {
		return nil, fmt.Errorf("cos presign: %w", err)
	}
	return &PresignedUpload{
		URL:       u.String(),
		Method:    http.MethodPut,
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: time.Now().Add(cosPresignTTL),
	}, nil
}

func (c *COS) PublicURL(key string) string { return c.publicBase + "/" + key }

func (c *COS) Delete(ctx context.Context, key string) error {
	resp, err := c.client.Object.Delete(ctx, key)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("cos delete: %w", err)
	}
	return nil
}

func (c *COS) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	var out []ObjectInfo
	marker := ""
	for {
		res, _, err := c.client.Bucket.Get(ctx, &cos.BucketGetOptions{Prefix: prefix, Marker: marker, MaxKeys: 1000})
		if err != nil {
			return nil, fmt.Errorf("cos list: %w", err)
		}
		for _, o := range res.Contents {
			mod, _ := time.Parse(time.RFC3339, o.LastModified)
			out = append(out, ObjectInfo{Key: o.Key, Size: o.Size, ModTime: mod, ContentType: ContentTypeFor(o.Key)})
		}
		if !res.IsTruncated || res.NextMarker == "" {
			return out, nil
		}
		marker = res.NextMarker
	}
}

func (c *COS) Stat(ctx context.Context, key string) (*ObjectInfo, error) {
	resp, err := c.client.Object.Head(ctx, key, nil)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("cos head: %w", err)
	}
	size, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	mod, _ := http.ParseTime(resp.Header.Get("Last-Modified"))
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = ContentTypeFor(key)
	}
	return &ObjectInfo{Key: key, Size: size, ModTime: mod, ContentType: ct}, nil
}
