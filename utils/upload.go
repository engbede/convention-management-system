package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func init() {
	_ = mime.AddExtensionType(".mp4", "video/mp4")
	_ = mime.AddExtensionType(".webm", "video/webm")
	_ = mime.AddExtensionType(".mov", "video/quicktime")
	_ = mime.AddExtensionType(".mkv", "video/x-matroska")
	_ = mime.AddExtensionType(".avi", "video/x-msvideo")
}

var allowedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
}

var allowedVideoExtensions = map[string]bool{
	".mp4":  true,
	".webm": true,
	".mov":  true,
	".mkv":  true,
	".avi":  true,
}

// GetUploadBaseDir returns the configured upload root directory (default: "uploads").
// Supports Render persistent disks or environment variables UPLOADS_DIR / STORAGE_PATH.
func GetUploadBaseDir() string {
	if dir := strings.TrimSpace(os.Getenv("UPLOADS_DIR")); dir != "" {
		return dir
	}
	if dir := strings.TrimSpace(os.Getenv("STORAGE_PATH")); dir != "" {
		return dir
	}
	if fi, err := os.Stat("/var/data/uploads"); err == nil && fi.IsDir() {
		return "/var/data/uploads"
	}
	return "uploads"
}

// GetMaxVideoUploadSize returns the maximum allowed video upload size in bytes.
// Defaults to 500 MB (500 * 1024 * 1024 bytes) to accommodate full church programmes,
// worship conventions, sermons, and Bible studies.
// Can be customized via MAX_VIDEO_SIZE_MB environment variable.
func GetMaxVideoUploadSize() int64 {
	if mbStr := strings.TrimSpace(os.Getenv("MAX_VIDEO_SIZE_MB")); mbStr != "" {
		if mb, err := strconv.ParseInt(mbStr, 10, 64); err == nil && mb > 0 {
			return mb * 1024 * 1024
		}
	}
	return 500 * 1024 * 1024 // 500 MB default
}

// GetMaxVideoUploadSizeMB returns the maximum allowed video upload size in megabytes.
func GetMaxVideoUploadSizeMB() int64 {
	return GetMaxVideoUploadSize() / (1024 * 1024)
}

func resolveDestination(destination string) string {
	base := GetUploadBaseDir()
	if base == "uploads" {
		return destination
	}
	if destination == "uploads" {
		return base
	}
	if strings.HasPrefix(destination, "uploads/") {
		return filepath.Join(base, strings.TrimPrefix(destination, "uploads/"))
	}
	if strings.HasPrefix(destination, "uploads\\") {
		return filepath.Join(base, strings.TrimPrefix(destination, "uploads\\"))
	}
	return destination
}

func SaveUploadedFile(
	file multipart.File,
	header *multipart.FileHeader,
	destination string,
) (string, error) {
	destination = resolveDestination(destination)


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
		_ = os.Remove(path)
		return "", err
	}

	// Mirror to S3-compatible cloud object storage when configured
	if IsObjectStorageEnabled() {
		if f, oErr := os.Open(path); oErr == nil {
			defer f.Close()
			s3Key := filename
			if strings.Contains(destination, "profiles") {
				s3Key = "profiles/" + filename
			} else if strings.Contains(destination, "covers") {
				s3Key = "covers/" + filename
			} else if strings.Contains(destination, "posts") {
				s3Key = "posts/" + filename
			}
			_, _ = UploadToObjectStorage(s3Key, f, header.Size, contentType)
		}
	}

	return filename, nil
}

// SaveUploadedVideo validates and stores an uploaded video file for community posts.
// Supports mp4, webm, mov, mkv, and avi videos up to the configured limit (default 500 MB).
func SaveUploadedVideo(
	file multipart.File,
	header *multipart.FileHeader,
	destination string,
) (string, error) {
	if file == nil || header == nil {
		return "", errors.New("no video file provided")
	}
	defer file.Close()

	destination = resolveDestination(destination)

	maxFileSize := GetMaxVideoUploadSize()

	if header.Size <= 0 {
		return "", errors.New("video file is empty")
	}

	if header.Size > maxFileSize {
		return "", fmt.Errorf("video size must not exceed %d MB", GetMaxVideoUploadSizeMB())
	}

	ext := strings.ToLower(
		filepath.Ext(header.Filename),
	)

	if !allowedVideoExtensions[ext] {
		return "", errors.New("only mp4, webm, mov, mkv and avi videos are allowed")
	}

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}

	contentType := http.DetectContentType(buffer[:n])

	// Validate content type: must be video/* or binary application/octet-stream container
	if !strings.HasPrefix(contentType, "video/") && contentType != "application/octet-stream" {
		return "", errors.New("uploaded file is not a valid video")
	}

	// Reset file pointer before copying
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return "", err
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

	copyBuf := make([]byte, 64*1024)
	_, copyErr := io.CopyBuffer(dst, file, copyBuf)
	closeErr := dst.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return "", closeErr
	}

	// Mirror to S3-compatible cloud object storage when configured
	if IsObjectStorageEnabled() {
		if f, oErr := os.Open(path); oErr == nil {
			defer f.Close()
			s3Key := filename
			if strings.Contains(destination, "posts") {
				s3Key = "posts/" + filename
			}
			_, _ = UploadToObjectStorage(s3Key, f, header.Size, contentType)
		}
	}

	return filename, nil
}

func DeleteFile(path string) error {

	if path == "" {
		return nil
	}
	resolvedPath := resolveDestination(path)

	if IsObjectStorageEnabled() {
		s3Key := strings.TrimPrefix(path, "uploads/")
		s3Key = strings.TrimPrefix(s3Key, "/")
		_ = DeleteFromObjectStorage(s3Key)
	}

	if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
		return nil
	}

	return os.Remove(resolvedPath)
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
	destination = resolveDestination(destination)
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

