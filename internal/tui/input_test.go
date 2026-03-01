package tui

import (
	"testing"
)

func TestPngDimensions_ValidPNG(t *testing.T) {
	// Minimal 1x1 white PNG header bytes.
	data := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, // IHDR chunk length
		0x49, 0x48, 0x44, 0x52, // "IHDR"
		0x00, 0x00, 0x00, 0x01, // width: 1
		0x00, 0x00, 0x00, 0x01, // height: 1
		0x08, 0x02,             // bit depth, color type
		0x00, 0x00, 0x00,       // compression, filter, interlace
	}
	w, h := pngDimensions(data)
	if w != 1 || h != 1 {
		t.Errorf("pngDimensions = (%d, %d), want (1, 1)", w, h)
	}
}

func TestPngDimensions_LargerImage(t *testing.T) {
	// 1920x1080 PNG header.
	data := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x07, 0x80, // 1920
		0x00, 0x00, 0x04, 0x38, // 1080
		0x08, 0x02,
		0x00, 0x00, 0x00,
	}
	w, h := pngDimensions(data)
	if w != 1920 || h != 1080 {
		t.Errorf("pngDimensions = (%d, %d), want (1920, 1080)", w, h)
	}
}

func TestPngDimensions_TooShort(t *testing.T) {
	w, h := pngDimensions([]byte{0x89, 0x50})
	if w != 0 || h != 0 {
		t.Errorf("pngDimensions = (%d, %d), want (0, 0)", w, h)
	}
}

func TestPngDimensions_NotPNG(t *testing.T) {
	data := make([]byte, 30)
	data[0] = 0xFF // JPEG marker
	w, h := pngDimensions(data)
	if w != 0 || h != 0 {
		t.Errorf("pngDimensions = (%d, %d), want (0, 0)", w, h)
	}
}

func TestInputResetClearsPendingImages(t *testing.T) {
	m := NewInputModel(DefaultStyles())
	m.pendingImages = []PendingImage{{Data: []byte{1, 2, 3}}}
	m.Reset()
	if len(m.pendingImages) != 0 {
		t.Errorf("expected 0 pending images after reset, got %d", len(m.pendingImages))
	}
}

func TestInputHasImages(t *testing.T) {
	m := NewInputModel(DefaultStyles())
	if m.HasImages() {
		t.Error("expected no images initially")
	}
	m.pendingImages = []PendingImage{{Data: []byte{1}}}
	if !m.HasImages() {
		t.Error("expected images after staging")
	}
}

func TestSubmitMsgCarriesImages(t *testing.T) {
	images := []PendingImage{
		{Data: []byte{1}, Width: 100, Height: 200},
		{Data: []byte{2}, Width: 300, Height: 400},
	}
	msg := SubmitMsg{Text: "test", Images: images}
	if len(msg.Images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(msg.Images))
	}
	if msg.Images[0].Width != 100 {
		t.Errorf("first image width = %d, want 100", msg.Images[0].Width)
	}
}
