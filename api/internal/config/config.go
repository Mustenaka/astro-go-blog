// Package config reads the server configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// Config is the fully resolved server configuration.
type Config struct {
	Addr        string
	DataDir     string
	StorageKind string // local | cos

	COSBucket       string
	COSRegion       string
	COSSecretID     string
	COSSecretKey    string
	AssetPublicBase string

	AdminUser         string
	AdminPasswordHash string
	SessionSecret     string

	ContentDir    string
	SiteDir       string
	ReleaseScript string

	GitHubWebhookSecret string
	CORSOrigins         []string
	MainBranch          string
}

// Load reads api/.env (if present, development only) and then the process environment.
func Load() (*Config, error) {
	// Development convenience: a .env next to the working directory. Missing file is fine.
	_ = godotenv.Load()
	return FromEnv(os.Getenv)
}

// FromEnv builds a Config from a lookup function; tests pass their own.
func FromEnv(get func(string) string) (*Config, error) {
	c := &Config{
		Addr:                or(get("ADDR"), "127.0.0.1:8080"),
		DataDir:             or(get("DATA_DIR"), "./data"),
		StorageKind:         or(get("STORAGE_KIND"), "local"),
		COSBucket:           get("COS_BUCKET"),
		COSRegion:           get("COS_REGION"),
		COSSecretID:         get("COS_SECRET_ID"),
		COSSecretKey:        get("COS_SECRET_KEY"),
		AssetPublicBase:     strings.TrimRight(get("ASSET_PUBLIC_BASE"), "/"),
		AdminUser:           or(get("ADMIN_USER"), "admin"),
		AdminPasswordHash:   get("ADMIN_PASSWORD_HASH"),
		SessionSecret:       get("SESSION_SECRET"),
		ContentDir:          or(get("CONTENT_DIR"), "../content"),
		SiteDir:             or(get("SITE_DIR"), "../site"),
		ReleaseScript:       get("RELEASE_SCRIPT"),
		GitHubWebhookSecret: get("GITHUB_WEBHOOK_SECRET"),
		MainBranch:          or(get("MAIN_BRANCH"), "main"),
	}
	for _, o := range strings.Split(get("CORS_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.CORSOrigins = append(c.CORSOrigins, strings.TrimRight(o, "/"))
		}
	}
	if abs, err := filepath.Abs(c.DataDir); err == nil {
		c.DataDir = abs
	}
	if c.AssetPublicBase == "" && c.StorageKind == "local" {
		host := c.Addr
		if strings.HasPrefix(host, ":") || strings.HasPrefix(host, "0.0.0.0:") {
			host = "localhost" + host[strings.LastIndex(host, ":"):]
		}
		c.AssetPublicBase = "http://" + strings.Replace(host, "127.0.0.1", "localhost", 1) + "/assets"
	}
	return c, c.Validate()
}

// Validate checks the invariants that would otherwise surface as runtime failures.
func (c *Config) Validate() error {
	var errs []error
	switch c.StorageKind {
	case "local":
	case "cos":
		if c.COSBucket == "" || c.COSRegion == "" || c.COSSecretID == "" || c.COSSecretKey == "" || c.AssetPublicBase == "" {
			errs = append(errs, errors.New("STORAGE_KIND=cos requires COS_BUCKET, COS_REGION, COS_SECRET_ID, COS_SECRET_KEY and ASSET_PUBLIC_BASE"))
		}
	default:
		errs = append(errs, fmt.Errorf("STORAGE_KIND must be local or cos, got %q", c.StorageKind))
	}
	if len(c.SessionSecret) < 32 {
		errs = append(errs, errors.New("SESSION_SECRET must be at least 32 characters"))
	}
	if c.AdminPasswordHash == "" {
		errs = append(errs, errors.New("ADMIN_PASSWORD_HASH is required (generate with: server hash-password)"))
	}
	return errors.Join(errs...)
}

func or(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
