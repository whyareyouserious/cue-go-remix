// Package platform owns desktop capture; it never sends audio or images to a network.
package platform

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"math"
)

const sampleRate = 16000

// encodeScreenshot caps the image size before a screenshot leaves this package.
func encodeScreenshot(src image.Image) (string, error) {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		return "", errors.New("屏幕截图为空")
	}
	const longest = 1568
	if w > longest || h > longest {
		scale := math.Min(float64(longest)/float64(w), float64(longest)/float64(h))
		nw, nh := max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))
		dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
		// Bilinear interpolation keeps screenshot text legible without an extra dependency.
		for y := 0; y < nh; y++ {
			fy := (float64(y)+0.5)*float64(h)/float64(nh) - 0.5
			y0 := max(0, int(math.Floor(fy)))
			y1 := min(h-1, y0+1)
			vy := fy - float64(y0)
			for x := 0; x < nw; x++ {
				fx := (float64(x)+0.5)*float64(w)/float64(nw) - 0.5
				x0 := max(0, int(math.Floor(fx)))
				x1 := min(w-1, x0+1)
				vx := fx - float64(x0)
				c00 := color.RGBAModel.Convert(src.At(bounds.Min.X+x0, bounds.Min.Y+y0)).(color.RGBA)
				c10 := color.RGBAModel.Convert(src.At(bounds.Min.X+x1, bounds.Min.Y+y0)).(color.RGBA)
				c01 := color.RGBAModel.Convert(src.At(bounds.Min.X+x0, bounds.Min.Y+y1)).(color.RGBA)
				c11 := color.RGBAModel.Convert(src.At(bounds.Min.X+x1, bounds.Min.Y+y1)).(color.RGBA)
				mix := func(a, b, c, d uint8) uint8 {
					return uint8((float64(a)*(1-vx)+float64(b)*vx)*(1-vy) + (float64(c)*(1-vx)+float64(d)*vx)*vy)
				}
				dst.SetRGBA(x, y, color.RGBA{mix(c00.R, c10.R, c01.R, c11.R), mix(c00.G, c10.G, c01.G, c11.G), mix(c00.B, c10.B, c01.B, c11.B), 255})
			}
		}
		src = dst
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 75}); err != nil {
		return "", err
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// resampler integrates the source signal over each destination sample. State is
// retained across packets so frame boundaries do not change pitch or duration.
type resampler struct{ rate, remaining, weighted float64 }

func newResampler(rate int) *resampler {
	r := &resampler{rate: float64(rate) / sampleRate}
	r.remaining = r.rate
	return r
}
func (r *resampler) push(samples []float64) []int16 {
	out := make([]int16, 0, int(float64(len(samples))/r.rate)+1)
	for _, v := range samples {
		left := 1.0
		for left > 1e-9 {
			take := math.Min(left, r.remaining)
			r.weighted += v * take
			left -= take
			r.remaining -= take
			if r.remaining < 1e-9 {
				s := math.Max(-1, math.Min(1, r.weighted/r.rate))
				out = append(out, int16(math.Round(s*32767)))
				r.remaining, r.weighted = r.rate, 0
			}
		}
	}
	return out
}
