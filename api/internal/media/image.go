// Package media prepares images for upload: decode, limit the longest edge, encode as WebP.
package media

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"path"
	"strings"

	"github.com/gen2brain/webp"
	"golang.org/x/image/draw"
)

// DefaultMaxEdge is the AGENTS.md 5.4 limit for the longest side of an image.
const DefaultMaxEdge = 2000

// Options for PrepareImage.
type Options struct {
	// MaxEdge limits the longest side; 0 means DefaultMaxEdge.
	MaxEdge int
	// KeepOriginal skips conversion and resizing entirely.
	KeepOriginal bool
	// Quality for lossy WebP, 1-100; 0 means 82.
	Quality int
}

// Result describes the prepared file.
type Result struct {
	Data        []byte
	Filename    string
	ContentType string
	Converted   bool
	Width       int
	Height      int
}

var imageExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true}

// PrepareImage converts PNG/JPEG/GIF/WebP input to WebP no larger than MaxEdge. Non-image
// files and KeepOriginal pass through untouched (SVG is never converted).
func PrepareImage(data []byte, filename string, opts Options) (*Result, error) {
	ext := strings.ToLower(path.Ext(filename))
	if opts.KeepOriginal || !imageExt[ext] {
		return &Result{Data: data, Filename: filename, ContentType: contentTypeFor(ext)}, nil
	}
	if opts.MaxEdge <= 0 {
		opts.MaxEdge = DefaultMaxEdge
	}
	if opts.Quality <= 0 {
		opts.Quality = 82
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", filename, err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > opts.MaxEdge || h > opts.MaxEdge {
		scale := float64(opts.MaxEdge) / float64(max(w, h))
		nw, nh := max(1, int(float64(w)*scale+0.5)), max(1, int(float64(h)*scale+0.5))
		dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
		img, w, h = dst, nw, nh
	}
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, webp.Options{Quality: opts.Quality, Method: 4}); err != nil {
		return nil, fmt.Errorf("encode webp: %w", err)
	}
	name := strings.TrimSuffix(path.Base(filename), path.Ext(filename)) + ".webp"
	return &Result{Data: buf.Bytes(), Filename: name, ContentType: "image/webp", Converted: true, Width: w, Height: h}, nil
}

func contentTypeFor(ext string) string {
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".avif":
		return "image/avif"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".glb":
		return "model/gltf-binary"
	case ".gltf":
		return "model/gltf+json"
	case ".json":
		return "application/json"
	case ".html":
		return "text/html"
	case ".js":
		return "text/javascript"
	case ".css":
		return "text/css"
	case ".wasm":
		return "application/wasm"
	case ".woff2":
		return "font/woff2"
	case ".woff":
		return "font/woff"
	default:
		return "application/octet-stream"
	}
}
