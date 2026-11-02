package imageproc

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func testJPEG(t *testing.T, w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, x%h, color.RGBA{200, 10, 10, 255})
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withEXIF injects an APP1 Exif segment carrying a fake GPS marker right
// after the SOI marker.
func withEXIF(src []byte) []byte {
	payload := append([]byte("Exif\x00\x00"), []byte("GPSLatitude=50.8503;GPSLongitude=4.3517")...)
	seg := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}
	seg = append(seg, payload...)
	out := append([]byte{}, src[:2]...)
	out = append(out, seg...)
	return append(out, src[2:]...)
}

func TestProcessStripsEXIF(t *testing.T) {
	in := withEXIF(testJPEG(t, 64, 48))
	if !bytes.Contains(in, []byte("GPSLatitude")) {
		t.Fatal("fixture should contain GPS data")
	}
	res, err := Process(in)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(res.Data, []byte("GPSLatitude")) || bytes.Contains(res.Data, []byte("Exif")) {
		t.Fatal("EXIF data survived re-encoding")
	}
	if res.Width != 64 || res.Height != 48 || res.ContentType != "image/jpeg" {
		t.Fatalf("unexpected result %+v", res)
	}
}

func TestProcessResizesAndConvertsPNG(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3000, 1500))
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	res, err := Process(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != MaxOutputEdge || res.Height != 1024 || Sniff(res.Data) != "jpeg" {
		t.Fatalf("unexpected %dx%d %s", res.Width, res.Height, Sniff(res.Data))
	}
}

// Scenario 13: SVG, HTML and polyglots are rejected.
func TestRejects(t *testing.T) {
	cases := map[string][]byte{
		"svg":       []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"html":      []byte("<html><body>hi</body></html>"),
		"truncated": testJPEG(t, 32, 32)[:40],
		"polyglot":  append([]byte{0xFF, 0xD8, 0xFF}, []byte("<script>alert(1)</script>")...),
		"empty":     {},
	}
	for name, data := range cases {
		if _, err := Process(data); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
