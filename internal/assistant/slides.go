package assistant

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/jpeg"
	"math/bits"
	"strings"
)

// SlideFingerprint ignores small cursor/clock changes while retaining slide layout.
func SlideFingerprint(dataURL string) ([4]uint64, error) {
	var hash [4]uint64
	_, data, _ := strings.Cut(dataURL, ",")
	raw, e := base64.StdEncoding.DecodeString(data)
	if e != nil {
		return hash, e
	}
	img, _, e := image.Decode(bytes.NewReader(raw))
	if e != nil {
		return hash, e
	}
	b := img.Bounds()
	var values [256]uint32
	var sum uint64
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			r, g, bl, _ := img.At(b.Min.X+(2*x+1)*b.Dx()/32, b.Min.Y+(2*y+1)*b.Dy()/32).RGBA()
			v := (299*r + 587*g + 114*bl) / 1000
			values[y*16+x] = v
			sum += uint64(v)
		}
	}
	mean := uint32(sum / 256)
	for i, v := range values {
		if v > mean {
			hash[i/64] |= 1 << uint(i%64)
		}
	}
	return hash, nil
}
func SlideDifference(a, b [4]uint64) int {
	n := 0
	for i := range a {
		n += bits.OnesCount64(a[i] ^ b[i])
	}
	return n
}
