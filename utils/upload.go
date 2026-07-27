package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
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

	const maxFileSize = 5 * 1024 * 1024 // 5 MB

	if header.Size > maxFileSize {
		return "", errors.New("image size must not exceed 5 MB")
	}

	buffer := make([]byte, 512)

	_, err := file.Read(buffer)
	if err != nil {
		return "", err
	}

	contentType := http.DetectContentType(buffer)

	if !strings.HasPrefix(contentType, "image/") {
		return "", errors.New("only image files are allowed")
	}

	// Reset file pointer before copying
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(
		filepath.Ext(header.Filename),
	)

	if !allowedExtensions[ext] {
		return "", errors.New("only jpg, jpeg, png and webp images are allowed")
	}

	random := make([]byte, 16)

	_, err = rand.Read(random)
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

func DeleteFile(path string) error {

	if path == "" {
		return nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	return os.Remove(path)
}
