package utils

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"

	"github.com/skip2/go-qrcode"
)

// GenerateQRCodeBase64 generates a QR code for the given data and returns it as a base64 encoded string
// that can be used directly in an HTML img tag
func GenerateQRCodeBase64(data string, size int) (string, error) {
	// Generate QR code
	qr, err := qrcode.New(data, qrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("failed to generate QR code: %w", err)
	}

	// Create a buffer to store the PNG image
	buf := new(bytes.Buffer)

	// Get the image and encode it as PNG
	img := qr.Image(size)
	if err := png.Encode(buf, img); err != nil {
		return "", fmt.Errorf("failed to encode QR code as PNG: %w", err)
	}

	// Encode the PNG image as base64
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Return the base64 encoded image with the data URI scheme
	return "data:image/png;base64," + encoded, nil
}
