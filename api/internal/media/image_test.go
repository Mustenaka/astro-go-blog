package media

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/gen2brain/webp"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPrepareImageConvertsAndResizes(t *testing.T) {
	res, err := PrepareImage(pngBytes(t, 3000, 1500), "Shot.PNG", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Converted || res.ContentType != "image/webp" || res.Filename != "Shot.webp" {
		t.Fatalf("unexpected result %+v", res)
	}
	if res.Width != 2000 || res.Height != 1000 {
		t.Fatalf("expected 2000x1000, got %dx%d", res.Width, res.Height)
	}
	img, err := webp.Decode(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 2000 {
		t.Fatalf("decoded width %d", img.Bounds().Dx())
	}
}

func TestPrepareImageKeepsSmallDimensions(t *testing.T) {
	res, err := PrepareImage(pngBytes(t, 640, 480), "a.png", Options{MaxEdge: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != 640 || res.Height != 480 || !res.Converted {
		t.Fatalf("unexpected %+v", res)
	}
}

func TestPrepareImagePassThrough(t *testing.T) {
	data := pngBytes(t, 10, 10)
	res, err := PrepareImage(data, "a.png", Options{KeepOriginal: true})
	if err != nil || res.Converted || res.ContentType != "image/png" || !bytes.Equal(res.Data, data) {
		t.Fatalf("keepOriginal: %+v %v", res, err)
	}
	res, err = PrepareImage([]byte("<svg/>"), "icon.svg", Options{})
	if err != nil || res.Converted || res.ContentType != "image/svg+xml" {
		t.Fatalf("svg must pass through: %+v %v", res, err)
	}
	if _, err := PrepareImage([]byte("not an image"), "x.png", Options{}); err == nil {
		t.Fatal("garbage png must fail")
	}
}
