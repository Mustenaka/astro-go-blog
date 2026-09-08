// Command mcp is the stdio MCP server that lets AI tools operate the blog: read and write
// posts and works, upload assets, validate content, trigger builds and read stats.
//
// It reads the same api/.env as the HTTP server and shares its database and storage.
// stdout carries the protocol; all logging goes to stderr.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Mustenaka/astro-go-blog/api/internal/build"
	"github.com/Mustenaka/astro-go-blog/api/internal/config"
	"github.com/Mustenaka/astro-go-blog/api/internal/content"
	"github.com/Mustenaka/astro-go-blog/api/internal/db"
	"github.com/Mustenaka/astro-go-blog/api/internal/mcpserver"
	"github.com/Mustenaka/astro-go-blog/api/internal/storage"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("mcp server failed", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	database, err := db.Open(filepath.Join(cfg.DataDir, "blog.db"))
	if err != nil {
		return err
	}
	defer database.Close()

	var store storage.Storage
	switch cfg.StorageKind {
	case "cos":
		store, err = storage.NewCOS(storage.COSConfig{Bucket: cfg.COSBucket, Region: cfg.COSRegion, SecretID: cfg.COSSecretID, SecretKey: cfg.COSSecretKey, PublicBase: cfg.AssetPublicBase})
	default:
		origin := strings.TrimSuffix(cfg.AssetPublicBase, "/assets")
		store, err = storage.NewLocal(filepath.Join(cfg.DataDir, "assets"), cfg.AssetPublicBase, origin+"/_local/upload")
	}
	if err != nil {
		return err
	}

	contentStore, err := content.NewStore(cfg.ContentDir)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runner := build.New(database, cfg.ReleaseScript, filepath.Join(cfg.DataDir, "builds"))
	runner.Env = []string{"SITE_DIR=" + cfg.SiteDir, "CONTENT_DIR=" + cfg.ContentDir}
	runner.Start(ctx)

	server := mcpserver.New(mcpserver.Deps{
		Config:  cfg,
		DB:      database,
		Storage: store,
		Content: contentStore,
		Builds:  runner,
		SiteDir: cfg.SiteDir,
		Logger:  logger,
	})
	logger.Info("mcp server starting", "content", contentStore.Root, "storage", cfg.StorageKind)
	return server.Run(ctx, &mcp.StdioTransport{})
}
