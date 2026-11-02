// Package imageproc validates and re-encodes uploaded photos: the format is
// detected from magic bytes (never from the file name or the client MIME
// type), the image is fully decoded, downscaled and re-encoded as JPEG, which
// drops every metadata block (EXIF, GPS, XMP, ICC comments...).
package imageproc

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

const (
	MaxInputPixels = 8000
	MaxOutputEdge  = 2048
	JPEGQuality    = 85
)

var (
	ErrUnsupportedFormat = errors.New("unsupported image format")
	ErrTooLarge          = errors.New("image dimensions too large")
	ErrCorrupt           = errors.New("image cannot be decoded")
)

type Result struct {
	Data        []byte
	Width       int
	Height      int
	ContentType string
}

// Sniff returns "jpeg", "png", "webp" or "" from the leading bytes.
func Sniff(b []byte) string {
	switch {
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "jpeg"
	case len(b) >= 8 && bytes.Equal(b[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "png"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "webp"
	}
	return ""
}

func Process(data []byte) (*Result, error) {
	format := Sniff(data)
	var (
		decodeConfig func([]byte) (image.Config, error)
		decode       func([]byte) (image.Image, error)
	)
	switch format {
	case "jpeg":
		decodeConfig = func(b []byte) (image.Config, error) { return jpeg.DecodeConfig(bytes.NewReader(b)) }
		decode = func(b []byte) (image.Image, error) { return jpeg.Decode(bytes.NewReader(b)) }
	case "png":
		decodeConfig = func(b []byte) (image.Config, error) { return png.DecodeConfig(bytes.NewReader(b)) }
		decode = func(b []byte) (image.Image, error) { return png.Decode(bytes.NewReader(b)) }
	case "webp":
		decodeConfig = func(b []byte) (image.Config, error) { return webp.DecodeConfig(bytes.NewReader(b)) }
		decode = func(b []byte) (image.Image, error) { return webp.Decode(bytes.NewReader(b)) }
	default:
		return nil, ErrUnsupportedFormat
	}
	cfg, err := decodeConfig(data)
	if err != nil {
		return nil, ErrCorrupt
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, ErrCorrupt
	}
	if cfg.Width > MaxInputPixels || cfg.Height > MaxInputPixels {
		return nil, ErrTooLarge
	}
	img, err := decode(data)
	if err != nil {
		return nil, ErrCorrupt
	}
	img = resize(img, MaxOutputEdge)
	// Flatten onto an opaque RGBA canvas (JPEG has no alpha channel).
	b := img.Bounds()
	canvas := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(canvas, canvas.Bounds(), img, b.Min, draw.Over)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, canvas, &jpeg.Options{Quality: JPEGQuality}); err != nil {
		return nil, err
	}
	return &Result{Data: out.Bytes(), Width: b.Dx(), Height: b.Dy(), ContentType: "image/jpeg"}, nil
}

func resize(img image.Image, maxEdge int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxEdge && h <= maxEdge {
		return img
	}
	if w >= h {
		h = h * maxEdge / w
		w = maxEdge
	} else {
		w = w * maxEdge / h
		h = maxEdge
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Over, nil)
	return dst
}
