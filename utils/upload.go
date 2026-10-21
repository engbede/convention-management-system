package utils

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// MediaStorageSaverFunc persists media bytes into a durable store (such as PostgreSQL).
type MediaStorageSaverFunc func(mediaKey string, contentType string, data []byte) error

// MediaStorageDeleterFunc removes media from the durable store.
type MediaStorageDeleterFunc func(mediaKey string) error

var (
	MediaStorageSaver   MediaStorageSaverFunc
	MediaStorageDeleter MediaStorageDeleterFunc
)

// SetMediaStorageHooks registers durable persistence callbacks.
func SetMediaStorageHooks(saver MediaStorageSaverFunc, deleter MediaStorageDeleterFunc) {
	MediaStorageSaver = saver
	MediaStorageDeleter = deleter
}

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
// Supports Render persistent disks or environment variables UPLOADS_DIR / STORAGE_PATH / RENDER_DISK_PATH / DATA_DIR.
func GetUploadBaseDir() string {
	if dir := strings.TrimSpace(os.Getenv("UPLOADS_DIR")); dir != "" {
		return dir
	}
	if dir := strings.TrimSpace(os.Getenv("STORAGE_PATH")); dir != "" {
		return dir
	}
	if dir := strings.TrimSpace(os.Getenv("RENDER_DISK_PATH")); dir != "" {
		return dir
	}
	if dir := strings.TrimSpace(os.Getenv("DATA_DIR")); dir != "" {
		return dir
	}
	if fi, err := os.Stat("/var/data"); err == nil && fi.IsDir() {
		_ = os.MkdirAll("/var/data/uploads", 0755)
		return "/var/data/uploads"
	}
	if fi, err := os.Stat("/var/data/uploads"); err == nil && fi.IsDir() {
		return "/var/data/uploads"
	}
	return "uploads"
}

// GetMaxVideoUploadSize returns the maximum allowed video upload size in bytes.
// Defaults to 10 GB (10240 * 1024 * 1024 bytes) to accommodate complete 4–5+ hour church programmes,
// worship conventions, services, and Bible studies across full HD (1080p) and standard HD (720p).
// Can be customized via MAX_VIDEO_SIZE_MB environment variable.
func GetMaxVideoUploadSize() int64 {
	if mbStr := strings.TrimSpace(os.Getenv("MAX_VIDEO_SIZE_MB")); mbStr != "" {
		if mb, err := strconv.ParseInt(mbStr, 10, 64); err == nil && mb > 0 {
			return mb * 1024 * 1024
		}
	}
	return 10240 * 1024 * 1024 // 10 GB default
}

// GetMaxVideoUploadSizeMB returns the maximum allowed video upload size in megabytes.
func GetMaxVideoUploadSizeMB() int64 {
	return GetMaxVideoUploadSize() / (1024 * 1024)
}

// GetMaxDurableVideoSize returns the maximum video size in bytes that will be mirrored
// to PostgreSQL durable database storage (default: 200 MB, configurable via MAX_DURABLE_VIDEO_MB).
func GetMaxDurableVideoSize() int64 {
	if mbStr := strings.TrimSpace(os.Getenv("MAX_DURABLE_VIDEO_MB")); mbStr != "" {
		if mb, err := strconv.ParseInt(mbStr, 10, 64); err == nil && mb > 0 {
			return mb * 1024 * 1024
		}
	}
	return 200 * 1024 * 1024 // 200 MB default
}

// FormatBytes formats byte count into human-readable representation (B, KB, MB, GB).
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
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

	const maxFileSize = 10 * 1024 * 1024 // 10 MB

	if header.Size > maxFileSize {
		return "", errors.New("image size must not exceed 10 MB")
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

	var data []byte
	if f, rErr := os.Open(path); rErr == nil {
		data, _ = io.ReadAll(f)
		_ = f.Close()
	}

	s3Key := filename
	if strings.Contains(destination, "profiles") {
		s3Key = "profiles/" + filename
	} else if strings.Contains(destination, "covers") {
		s3Key = "covers/" + filename
	} else if strings.Contains(destination, "posts") {
		s3Key = "posts/" + filename
	} else if strings.Contains(destination, "chat") {
		s3Key = "chat/" + filename
	}

	// Mirror to S3-compatible cloud object storage when configured
	if IsObjectStorageEnabled() && len(data) > 0 {
		if _, uErr := UploadToObjectStorage(s3Key, bytes.NewReader(data), int64(len(data)), contentType); uErr != nil {
			log.Printf("[Storage] S3 upload error for %s: %v", s3Key, uErr)
		}
	}

	// Persist to durable database media storage
	if MediaStorageSaver != nil && len(data) > 0 {
		if sErr := MediaStorageSaver(s3Key, contentType, data); sErr != nil {
			log.Printf("[Storage] Durable media saver error for %s: %v", s3Key, sErr)
		}
	}

	return filename, nil
}

// SaveUploadedVideo validates and stores an uploaded video file for community posts.
// Supports mp4, webm, mov, mkv, and avi videos up to the configured limit (default 10 GB).
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

	s3Key := filename
	if strings.Contains(destination, "posts") {
		s3Key = "posts/" + filename
	}

	// Persist video files up to max durable size (default 200 MB) to durable database media storage so they survive Render redeployments
	if MediaStorageSaver != nil && header.Size <= GetMaxDurableVideoSize() {
		if f, rErr := os.Open(path); rErr == nil {
			if vData, err := io.ReadAll(f); err == nil && len(vData) > 0 {
				if sErr := MediaStorageSaver(s3Key, contentType, vData); sErr != nil {
					log.Printf("[Storage] Durable media saver error for video %s: %v", s3Key, sErr)
				}
			}
			_ = f.Close()
		}
	}

	// Mirror to S3-compatible cloud object storage when configured
	if IsObjectStorageEnabled() {
		if f, oErr := os.Open(path); oErr == nil {
			defer f.Close()
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

	s3Key := strings.TrimPrefix(path, "uploads/")
	s3Key = strings.TrimPrefix(s3Key, "/")

	if IsObjectStorageEnabled() {
		_ = DeleteFromObjectStorage(s3Key)
	}

	if MediaStorageDeleter != nil {
		_ = MediaStorageDeleter(s3Key)
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
	".mp4": "video",
	".mov": "video",
	".avi": "video",
	".mkv": "video",
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

	copyBuf := make([]byte, 64*1024)
	_, copyErr := io.CopyBuffer(dst, file, copyBuf)
	closeErr := dst.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return "", "", "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return "", "", "", closeErr
	}

	var data []byte
	if f, rErr := os.Open(path); rErr == nil {
		data, _ = io.ReadAll(f)
		_ = f.Close()
	}

	mediaKey := "chat/" + filename
	if IsObjectStorageEnabled() && len(data) > 0 {
		_, _ = UploadToObjectStorage(mediaKey, bytes.NewReader(data), int64(len(data)), detectedType)
	}

	if MediaStorageSaver != nil && len(data) > 0 {
		_ = MediaStorageSaver(mediaKey, detectedType, data)
	}

	return filename, msgType, detectedType, nil
}
