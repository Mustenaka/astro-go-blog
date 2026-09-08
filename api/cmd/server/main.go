// Command server runs the blog backend.
//
//	server                 run the HTTP server (reads api/.env in development)
//	server hash-password   print an argon2id hash for ADMIN_PASSWORD_HASH
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Mustenaka/astro-go-blog/api/internal/auth"
	"github.com/Mustenaka/astro-go-blog/api/internal/build"
	"github.com/Mustenaka/astro-go-blog/api/internal/config"
	"github.com/Mustenaka/astro-go-blog/api/internal/db"
	"github.com/Mustenaka/astro-go-blog/api/internal/httpapi"
	"github.com/Mustenaka/astro-go-blog/api/internal/storage"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "hash-password" {
		os.Exit(hashPassword(os.Args[2:]))
	}
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func hashPassword(args []string) int {
	var password string
	if len(args) > 0 {
		password = args[0]
	} else {
		fmt.Fprint(os.Stderr, "password: ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			fmt.Fprintln(os.Stderr, "no password given")
			return 2
		}
		password = strings.TrimRight(line, "\r\n")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	fmt.Println(hash)
	return 0
}

func run() error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	database, err := db.Open(filepath.Join(cfg.DataDir, "blog.db"))
	if err != nil {
		return err
	}
	defer database.Close()

	store, err := newStorage(cfg)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runner := build.New(database, cfg.ReleaseScript, filepath.Join(cfg.DataDir, "builds"))
	runner.Env = []string{"SITE_DIR=" + cfg.SiteDir, "CONTENT_DIR=" + cfg.ContentDir}
	runner.Start(ctx)

	srv := httpapi.New(cfg, database, store, runner, httpapi.Options{Logger: logger})
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}

	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = auth.NewSessions(database).Purge(ctx)
			}
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	logger.Info("server listening", "addr", cfg.Addr, "storage", cfg.StorageKind, "data", cfg.DataDir, "assets", cfg.AssetPublicBase)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	logger.Info("server stopped")
	return nil
}

func newStorage(cfg *config.Config) (storage.Storage, error) {
	switch cfg.StorageKind {
	case "local":
		origin := strings.TrimSuffix(cfg.AssetPublicBase, "/assets")
		return storage.NewLocal(filepath.Join(cfg.DataDir, "assets"), cfg.AssetPublicBase, origin+"/_local/upload")
	case "cos":
		return storage.NewCOS(storage.COSConfig{
			Bucket: cfg.COSBucket, Region: cfg.COSRegion,
			SecretID: cfg.COSSecretID, SecretKey: cfg.COSSecretKey,
			PublicBase: cfg.AssetPublicBase,
		})
	default:
		return nil, fmt.Errorf("unknown STORAGE_KIND %q", cfg.StorageKind)
	}
}
