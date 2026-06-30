package helpers

import (
	"encoding/base64"

	"github.com/skip2/go-qrcode"
)

func GenerateQRCode(data string) (string, error) {

	png, err := qrcode.Encode(
		data,
		qrcode.Medium,
		220,
	)

	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(png), nil
}
