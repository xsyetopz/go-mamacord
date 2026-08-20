package executor

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestImageDimsVerifiesAttachmentMetadataAgainstContent(t *testing.T) {
	var data bytes.Buffer
	source := image.NewRGBA(image.Rect(0, 0, 2, 3))
	source.Set(0, 0, color.White)
	if err := png.Encode(&data, source); err != nil {
		t.Fatal(err)
	}
	if width, height, ok := imageDims(2, 3, data.Bytes()); !ok || width != 2 || height != 3 {
		t.Fatalf("valid image width=%d height=%d ok=%v", width, height, ok)
	}
	if _, _, ok := imageDims(200, 300, data.Bytes()); ok {
		t.Fatal("trusted false attachment dimensions")
	}
	if _, _, ok := imageDims(2, 3, []byte("not an image")); ok {
		t.Fatal("accepted invalid image")
	}
}
