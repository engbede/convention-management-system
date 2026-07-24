package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
)

var allowedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

func SaveUploadedFile(
	file multipart.File,
	header *multipart.FileHeader,
	destination string,
) (string, error) {

	defer file.Close()

	ext := strings.ToLower(
		filepath.Ext(header.Filename),
	)

	if !allowedExtensions[ext] {
		return "", errors.New("only jpg, jpeg, png and webp images are allowed")
	}

	random := make([]byte, 16)

	_, err := rand.Read(random)
	if err != nil {
		return "", err
	}

	filename := hex.EncodeToString(random) + ext

	path := filepath.Join(
		destination,
		filename,
	)

	dst, err := os.Create(path)
	if err != nil {
		return "", err
	}

	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		return "", err
	}

	return filename, nil
}
