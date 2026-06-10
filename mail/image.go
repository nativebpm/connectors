package mail

import (
	"bytes"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strings"
)

// hasTransparency checks if any pixel in the image is semi-transparent.
func hasTransparency(img image.Image) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a < 65535 {
				return true
			}
		}
	}
	return false
}

// resizeBilinear resizes an image using bilinear interpolation.
func resizeBilinear(img image.Image, w, h int) image.Image {
	srcBounds := img.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcX := float64(x) * float64(srcW) / float64(w)
			srcY := float64(y) * float64(srcH) / float64(h)

			x0 := int(srcX)
			y0 := int(srcY)
			x1 := x0 + 1
			if x1 >= srcW {
				x1 = x0
			}
			y1 := y0 + 1
			if y1 >= srcH {
				y1 = y0
			}

			dx := srcX - float64(x0)
			dy := srcY - float64(y0)

			c00 := img.At(srcBounds.Min.X+x0, srcBounds.Min.Y+y0)
			c10 := img.At(srcBounds.Min.X+x1, srcBounds.Min.Y+y0)
			c01 := img.At(srcBounds.Min.X+x0, srcBounds.Min.Y+y1)
			c11 := img.At(srcBounds.Min.X+x1, srcBounds.Min.Y+y1)

			r00, g00, b00, a00 := c00.RGBA()
			r10, g10, b10, a10 := c10.RGBA()
			r01, g01, b01, a01 := c01.RGBA()
			r11, g11, b11, a11 := c11.RGBA()

			r := float64(r00)*(1-dx)*(1-dy) + float64(r10)*dx*(1-dy) + float64(r01)*(1-dx)*dy + float64(r11)*dx*dy
			g := float64(g00)*(1-dx)*(1-dy) + float64(g10)*dx*(1-dy) + float64(g01)*(1-dx)*dy + float64(g11)*dx*dy
			b := float64(b00)*(1-dx)*(1-dy) + float64(b10)*dx*(1-dy) + float64(b01)*(1-dx)*dy + float64(b11)*dx*dy
			a := float64(a00)*(1-dx)*(1-dy) + float64(a10)*dx*(1-dy) + float64(a01)*(1-dx)*dy + float64(a11)*dx*dy

			dst.Set(x, y, color.RGBA64{
				R: uint16(r),
				G: uint16(g),
				B: uint16(b),
				A: uint16(a),
			})
		}
	}
	return dst
}

// optimizeImage decodes, resizes (if width > 600), and compresses PNG/JPEG images to minimize email size.
func optimizeImage(data []byte, filename string, contentType string) ([]byte, string, string) {
	isPNG := contentType == "image/png" || strings.HasSuffix(strings.ToLower(filename), ".png")
	isJPEG := contentType == "image/jpeg" || contentType == "image/jpg" ||
		strings.HasSuffix(strings.ToLower(filename), ".jpg") || strings.HasSuffix(strings.ToLower(filename), ".jpeg")

	if !isPNG && !isJPEG {
		return data, filename, contentType
	}

	// Only optimize if file size is > 100KB
	if len(data) <= 100*1024 {
		return data, filename, contentType
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, filename, contentType
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	targetW := width
	targetH := height

	if width > 600 {
		targetW = 600
		targetH = height * 600 / width
	}

	var resizedImg image.Image = img
	if targetW != width {
		resizedImg = resizeBilinear(img, targetW, targetH)
	}

	var buf bytes.Buffer
	outContentType := contentType
	outFilename := filename

	if isPNG && hasTransparency(resizedImg) {
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		if err := encoder.Encode(&buf, resizedImg); err != nil {
			return data, filename, contentType
		}
		outContentType = "image/png"
	} else {
		if err := jpeg.Encode(&buf, resizedImg, &jpeg.Options{Quality: 75}); err != nil {
			return data, filename, contentType
		}
		outContentType = "image/jpeg"
		ext := filepath.Ext(filename)
		if strings.ToLower(ext) != ".jpg" && strings.ToLower(ext) != ".jpeg" {
			outFilename = strings.TrimSuffix(filename, ext) + ".jpg"
		}
	}

	if buf.Len() < len(data) {
		return buf.Bytes(), outFilename, outContentType
	}

	return data, filename, contentType
}
