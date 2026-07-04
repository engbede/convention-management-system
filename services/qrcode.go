package services

import (
	"fmt"
	"os"

	"github.com/skip2/go-qrcode"
)

// GenerateQRCode creates a PNG QR code and returns its file path.
func GenerateQRCode(registrationNumber string) (string, error) {

	// Create the QR directory if it doesn't exist.
	err := os.MkdirAll("static/qr", os.ModePerm)
	if err != nil {
		return "", err
	}

	// Save the QR image.
	filename := fmt.Sprintf(
		"static/qr/%s.png",
		registrationNumber,
	)

	// Data encoded in the QR code.
	content := fmt.Sprintf(
		"http://localhost:8085/checkin?reg=%s",
		registrationNumber,
	)

	err = qrcode.WriteFile(
		content,
		qrcode.Medium,
		256,
		filename,
	)

	if err != nil {
		return "", err
	}

	return filename, nil
}
