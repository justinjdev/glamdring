package tui

import (
	"golang.design/x/clipboard"
)

// InitClipboard initializes the clipboard subsystem. Must be called once
// before any clipboard reads or writes. Returns an error if the clipboard
// is not available on this platform.
func InitClipboard() error {
	return clipboard.Init()
}

// ReadClipboardImage reads PNG image data from the system clipboard.
// Returns the raw PNG bytes and true if image data was found,
// or nil and false if the clipboard does not contain an image.
func ReadClipboardImage() ([]byte, bool) {
	data := clipboard.Read(clipboard.FmtImage)
	if len(data) == 0 {
		return nil, false
	}
	return data, true
}

// ReadClipboardText reads UTF-8 text from the system clipboard.
// Returns the text and true if text was found, or empty string and false.
func ReadClipboardText() (string, bool) {
	data := clipboard.Read(clipboard.FmtText)
	if len(data) == 0 {
		return "", false
	}
	return string(data), true
}

// WriteClipboardText writes UTF-8 text to the system clipboard.
func WriteClipboardText(text string) {
	clipboard.Write(clipboard.FmtText, []byte(text))
}
