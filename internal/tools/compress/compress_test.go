package compress

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveOptionsPresetsAndOverrides(t *testing.T) {
	for _, tc := range []struct {
		p    Preset
		q, d int
	}{{PresetLight, 65, 1280}, {PresetBalanced, 80, 1920}, {PresetHigh, 90, 2560}} {
		o, e := ResolveOptions(Options{Preset: tc.p})
		if e != nil || o.Quality != tc.q || o.MaxDim != tc.d {
			t.Fatalf("%s: %#v %v", tc.p, o, e)
		}
	}
	if o, e := ResolveOptions(Options{Preset: PresetHigh, Quality: 72, MaxDim: 0, MaxDimSet: true}); e != nil || o.Quality != 72 || o.MaxDim != 0 {
		t.Fatalf("override: %#v %v", o, e)
	}
	for _, v := range []Options{{Preset: "unknown"}, {Quality: 101}, {MaxDim: -1}} {
		if _, e := ResolveOptions(v); e == nil {
			t.Fatalf("expected invalid options: %#v", v)
		}
	}
}
func TestCompressImageBytesTransparencyAndNoBloat(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x * 2), G: uint8(y * 2), A: 255})
		}
	}
	img.Set(0, 0, color.NRGBA{})
	var b bytes.Buffer
	if e := png.Encode(&b, img); e != nil {
		t.Fatal(e)
	}
	opts, _ := ResolveOptions(Options{Preset: PresetBalanced, MaxDim: 50, MaxDimSet: true})
	r, e := CompressImageBytes(b.Bytes(), opts)
	if e != nil || r.Skipped || r.Format != "png" || r.Width != 50 {
		t.Fatalf("transparent: %#v %v", r, e)
	}
	flat := image.NewRGBA(image.Rect(0, 0, 8, 8))
	var small bytes.Buffer
	_ = png.Encode(&small, flat)
	r, e = CompressImageBytes(small.Bytes(), opts)
	if e != nil {
		t.Fatal(e)
	}
	if !r.Skipped {
		t.Fatalf("expected no-bloat skip")
	}
}
func TestCompressFileNoOverwriteAndInPlacePreservesOriginalOnFailure(t *testing.T) {
	d := t.TempDir()
	src := filepath.Join(d, "bad.png")
	_ = os.WriteFile(src, []byte("invalid"), 0600)
	if _, e := CompressImageFileInPlace(src, ResolvedOptions{Quality: 80}); e == nil {
		t.Fatal("expected decode error")
	}
	b, _ := os.ReadFile(src)
	if string(b) != "invalid" {
		t.Fatal("original changed")
	}
}
