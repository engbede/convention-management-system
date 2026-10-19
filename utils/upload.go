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

	if err := os.MkdirAll(destination, 0755); err != nil {
		return "", err
	}

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

var allowedChatExtensions = map[string]string{
	// Images -> "image"
	".jpg":  "image",
	".jpeg": "image",
	".png":  "image",
	".webp": "image",
	".gif":  "image",
	// Voice / Audio -> "voice"
	".mp3":  "voice",
	".wav":  "voice",
	".ogg":  "voice",
	".m4a":  "voice",
	".webm": "voice",
	".aac":  "voice",
	// Videos -> "video"
	".mp4":  "video",
	".mov":  "video",
	".avi":  "video",
	".mkv":  "video",
	// Documents / Files -> "file"
	".pdf":  "file",
	".doc":  "file",
	".docx": "file",
	".txt":  "file",
	".csv":  "file",
	".xlsx": "file",
	".xls":  "file",
	".zip":  "file",
}

// SaveChatAttachment validates and stores an attachment for chat messaging.
// Supports images, voice notes, videos, and documents up to 10MB.
func SaveChatAttachment(
	file multipart.File,
	header *multipart.FileHeader,
	destination string,
) (filename string, messageType string, contentType string, err error) {
	defer file.Close()

	const maxFileSize = 10 * 1024 * 1024 // 10 MB
	if header.Size <= 0 {
		return "", "", "", errors.New("attachment file is empty")
	}
	if header.Size > maxFileSize {
		return "", "", "", errors.New("attachment size must not exceed 10 MB")
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	msgType, allowed := allowedChatExtensions[ext]
	if !allowed {
		return "", "", "", errors.New("file format not supported for chat attachments")
	}

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", "", "", err
	}
	detectedType := http.DetectContentType(buffer)

	// Browser recorded audio in webm container often reports video/webm or application/octet-stream.
	// Normalize voice notes to standard audio MIME types so audio players load cleanly.
	if msgType == "voice" {
		if !strings.HasPrefix(detectedType, "audio/") {
			switch ext {
			case ".webm":
				detectedType = "audio/webm"
			case ".ogg":
				detectedType = "audio/ogg"
			case ".mp3":
				detectedType = "audio/mpeg"
			case ".wav":
				detectedType = "audio/wav"
			case ".m4a":
				detectedType = "audio/mp4"
			case ".aac":
				detectedType = "audio/aac"
			default:
				detectedType = "audio/webm"
			}
		}
	} else if msgType == "video" {
		if !strings.HasPrefix(detectedType, "video/") {
			switch ext {
			case ".mp4":
				detectedType = "video/mp4"
			case ".mov":
				detectedType = "video/quicktime"
			case ".avi":
				detectedType = "video/x-msvideo"
			case ".mkv":
				detectedType = "video/x-matroska"
			default:
				detectedType = "video/mp4"
			}
		}
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return "", "", "", err
	}

	random := make([]byte, 16)
	_, err = rand.Read(random)
	if err != nil {
		return "", "", "", err
	}

	filename = hex.EncodeToString(random) + ext

	if err := os.MkdirAll(destination, 0755); err != nil {
		return "", "", "", err
	}

	path := filepath.Join(destination, filename)
	dst, err := os.Create(path)
	if err != nil {
		return "", "", "", err
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		return "", "", "", err
	}

	return filename, msgType, detectedType, nil
}

