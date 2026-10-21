package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"convention-management-system/repository"
	"convention-management-system/utils"
)

// DefaultChunkSize is 10 MB per chunk, balancing network reliability, memory overhead, and HTTP part counts.
var DefaultChunkSize int64 = 10 * 1024 * 1024

// GetDefaultChunkSize returns the active chunk size in bytes, with optional CHUNK_SIZE_BYTES override.
func GetDefaultChunkSize() int64 {
	if csStr := strings.TrimSpace(os.Getenv("CHUNK_SIZE_BYTES")); csStr != "" {
		if cs, err := strconv.ParseInt(csStr, 10, 64); err == nil && cs > 0 {
			return cs
		}
	}
	return DefaultChunkSize
}

var sessionMu sync.Mutex

// ChunkedUploadSession holds metadata for an ongoing multipart/chunked video upload.
type ChunkedUploadSession struct {
	UploadID     string                  `json:"upload_id"`
	UserID       int                     `json:"user_id"`
	OriginalName string                  `json:"original_name"`
	TargetKey    string                  `json:"target_key"`
	FileSize     int64                   `json:"file_size"`
	ChunkSize    int64                   `json:"chunk_size"`
	TotalChunks  int                     `json:"total_chunks"`
	ContentType  string                  `json:"content_type"`
	CreatedAt    time.Time               `json:"created_at"`
	S3UploadID   string                  `json:"s3_upload_id,omitempty"`
	S3Parts      []utils.S3CompletedPart `json:"s3_parts,omitempty"`
}

func getTempChunksDir() string {
	base := utils.GetUploadBaseDir()
	dir := filepath.Join(base, "temp_chunks")
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func getSessionDir(uploadID string) string {
	return filepath.Join(getTempChunksDir(), filepath.Clean("/"+uploadID))
}

func getSessionMetadataPath(uploadID string) string {
	return filepath.Join(getSessionDir(uploadID), "session.json")
}

func saveSession(s *ChunkedUploadSession) error {
	path := getSessionMetadataPath(s.UploadID)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func loadSession(uploadID string) (*ChunkedUploadSession, error) {
	path := getSessionMetadataPath(uploadID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("upload session not found or expired: %w", err)
	}
	var s ChunkedUploadSession
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("corrupt upload session metadata: %w", err)
	}
	return &s, nil
}

// InitiateChunkedUpload initializes a resilient upload session for long-form video content up to 10 GB.
func InitiateChunkedUpload(userID int, filename string, fileSize int64, contentType string) (*ChunkedUploadSession, error) {
	if userID <= 0 {
		return nil, errors.New("unauthorized user")
	}

	maxAllowed := utils.GetMaxVideoUploadSize()
	if fileSize <= 0 {
		return nil, errors.New("video file is empty")
	}
	if fileSize > maxAllowed {
		return nil, fmt.Errorf("video size (%s) exceeds maximum allowed limit of %s", utils.FormatBytes(fileSize), utils.FormatBytes(maxAllowed))
	}

	ext := strings.ToLower(filepath.Ext(filename))
	allowedExts := map[string]bool{
		".mp4": true, ".webm": true, ".mov": true, ".mkv": true, ".avi": true,
	}
	if !allowedExts[ext] {
		return nil, fmt.Errorf("unsupported video format '%s'; only mp4, webm, mov, mkv, and avi are allowed", ext)
	}

	// Verify disk space on destination volume if not using cloud S3 storage
	if !utils.IsObjectStorageEnabled() {
		uploadBase := utils.GetUploadBaseDir()
		if freeSpace, err := utils.GetAvailableDiskSpace(uploadBase); err == nil && freeSpace > 0 {
			requiredSpace := uint64(fileSize) + (100 << 20) // file size + 100 MB buffer
			if freeSpace < requiredSpace {
				return nil, fmt.Errorf("insufficient server disk capacity: upload requires %s, but only %s is available. Please attach persistent disk or configure S3 object storage",
					utils.FormatBytes(fileSize), utils.FormatBytes(int64(freeSpace)))
			}
		}
	}

	rnd := make([]byte, 16)
	if _, err := rand.Read(rnd); err != nil {
		return nil, fmt.Errorf("failed generating session token: %w", err)
	}
	uploadID := hex.EncodeToString(rnd)

	rndName := make([]byte, 16)
	if _, err := rand.Read(rndName); err != nil {
		return nil, fmt.Errorf("failed generating file token: %w", err)
	}
	targetKey := "posts/" + hex.EncodeToString(rndName) + ext

	chunkSize := GetDefaultChunkSize()
	totalChunks := int((fileSize + chunkSize - 1) / chunkSize)
	if totalChunks <= 0 {
		totalChunks = 1
	}

	session := &ChunkedUploadSession{
		UploadID:     uploadID,
		UserID:       userID,
		OriginalName: filename,
		TargetKey:    targetKey,
		FileSize:     fileSize,
		ChunkSize:    chunkSize,
		TotalChunks:  totalChunks,
		ContentType:  contentType,
		CreatedAt:    time.Now().UTC(),
		S3Parts:      make([]utils.S3CompletedPart, 0),
	}

	sessionDir := getSessionDir(uploadID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed creating session workspace: %w", err)
	}

	// If cloud object storage is enabled, initiate multipart upload on S3 provider
	if utils.IsObjectStorageEnabled() {
		s3UploadID, err := utils.InitiateS3MultipartUpload(targetKey, contentType)
		if err != nil {
			log.Printf("[Upload] S3 multipart initiate failed (will fallback to disk staging): %v", err)
		} else {
			session.S3UploadID = s3UploadID
		}
	}

	if err := saveSession(session); err != nil {
		_ = os.RemoveAll(sessionDir)
		return nil, fmt.Errorf("failed persisting session metadata: %w", err)
	}

	return session, nil
}

// SaveChunk writes a single chunk, either directly to S3 multipart or onto temporary disk staging.
func SaveChunk(userID int, uploadID string, chunkIndex int, reader io.Reader, size int64) error {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	session, err := loadSession(uploadID)
	if err != nil {
		return err
	}

	if session.UserID != userID {
		return errors.New("unauthorized: upload session belongs to another user")
	}

	if chunkIndex < 0 || chunkIndex >= session.TotalChunks {
		return fmt.Errorf("invalid chunk index %d (total chunks: %d)", chunkIndex, session.TotalChunks)
	}

	// Direct S3 multipart upload if S3 session exists
	if session.S3UploadID != "" {
		partNum := chunkIndex + 1
		etag, s3Err := utils.UploadS3Part(session.TargetKey, session.S3UploadID, partNum, reader, size)
		if s3Err != nil {
			return fmt.Errorf("S3 chunk upload failed: %w", s3Err)
		}

		// Update or record ETag for this part
		updated := false
		for i, p := range session.S3Parts {
			if p.PartNumber == partNum {
				session.S3Parts[i].ETag = etag
				updated = true
				break
			}
		}
		if !updated {
			session.S3Parts = append(session.S3Parts, utils.S3CompletedPart{
				PartNumber: partNum,
				ETag:       etag,
			})
		}
		return saveSession(session)
	}

	// Local disk staging
	chunkPath := filepath.Join(getSessionDir(uploadID), fmt.Sprintf("chunk_%06d.part", chunkIndex))
	partFile, err := os.Create(chunkPath)
	if err != nil {
		return fmt.Errorf("failed creating chunk file: %w", err)
	}
	defer partFile.Close()

	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(partFile, reader, buf); err != nil {
		_ = os.Remove(chunkPath)
		return fmt.Errorf("failed streaming chunk data: %w", err)
	}

	return nil
}

// GetUploadedChunks returns a list of chunk indices successfully received for an active upload session.
func GetUploadedChunks(userID int, uploadID string) ([]int, error) {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	session, err := loadSession(uploadID)
	if err != nil {
		return nil, err
	}
	if session.UserID != userID {
		return nil, errors.New("unauthorized: session belongs to another user")
	}

	if session.S3UploadID != "" {
		result := make([]int, 0, len(session.S3Parts))
		for _, p := range session.S3Parts {
			result = append(result, p.PartNumber-1)
		}
		return result, nil
	}

	sessionDir := getSessionDir(uploadID)
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return nil, err
	}

	chunks := make([]int, 0, len(entries))
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "chunk_") && strings.HasSuffix(entry.Name(), ".part") {
			var idx int
			if _, sErr := fmt.Sscanf(entry.Name(), "chunk_%06d.part", &idx); sErr == nil {
				chunks = append(chunks, idx)
			}
		}
	}
	return chunks, nil
}

// CompleteChunkedUpload assembles all parts, verifies file integrity and size, and finalizes storage.
func CompleteChunkedUpload(userID int, uploadID string) (targetKey string, finalURL string, err error) {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	session, err := loadSession(uploadID)
	if err != nil {
		return "", "", err
	}
	if session.UserID != userID {
		return "", "", errors.New("unauthorized: session belongs to another user")
	}

	sessionDir := getSessionDir(uploadID)
	defer os.RemoveAll(sessionDir)

	// Finalize S3 multipart if active
	if session.S3UploadID != "" {
		if len(session.S3Parts) < session.TotalChunks {
			return "", "", fmt.Errorf("incomplete upload: received %d of %d parts", len(session.S3Parts), session.TotalChunks)
		}
		loc, s3Err := utils.CompleteS3MultipartUpload(session.TargetKey, session.S3UploadID, session.S3Parts)
		if s3Err != nil {
			return "", "", fmt.Errorf("failed completing S3 multipart upload: %w", s3Err)
		}
		return session.TargetKey, loc, nil
	}

	// Finalize disk-staged chunks
	uploadBase := utils.GetUploadBaseDir()
	finalDestPath := filepath.Join(uploadBase, session.TargetKey)
	if err := os.MkdirAll(filepath.Dir(finalDestPath), 0755); err != nil {
		return "", "", fmt.Errorf("failed creating target media directory: %w", err)
	}

	destFile, err := os.Create(finalDestPath)
	if err != nil {
		return "", "", fmt.Errorf("failed creating final target file: %w", err)
	}

	buf := make([]byte, 64*1024)
	var totalWritten int64

	for i := 0; i < session.TotalChunks; i++ {
		chunkPath := filepath.Join(sessionDir, fmt.Sprintf("chunk_%06d.part", i))
		cFile, oErr := os.Open(chunkPath)
		if oErr != nil {
			destFile.Close()
			_ = os.Remove(finalDestPath)
			return "", "", fmt.Errorf("missing chunk %d of %d: %w", i, session.TotalChunks, oErr)
		}

		n, cErr := io.CopyBuffer(destFile, cFile, buf)
		cFile.Close()
		if cErr != nil {
			destFile.Close()
			_ = os.Remove(finalDestPath)
			return "", "", fmt.Errorf("error concatenating chunk %d: %w", i, cErr)
		}
		totalWritten += n
	}

	if err := destFile.Close(); err != nil {
		_ = os.Remove(finalDestPath)
		return "", "", fmt.Errorf("failed closing finalized file: %w", err)
	}

	// Size verification
	if totalWritten != session.FileSize {
		_ = os.Remove(finalDestPath)
		return "", "", fmt.Errorf("final size mismatch: expected %d bytes, wrote %d bytes", session.FileSize, totalWritten)
	}

	// Format & integrity verification
	verifyFile, err := os.Open(finalDestPath)
	if err == nil {
		magic := make([]byte, 512)
		n, _ := verifyFile.Read(magic)
		verifyFile.Close()

		detected := http.DetectContentType(magic[:n])
		if !strings.HasPrefix(detected, "video/") && detected != "application/octet-stream" {
			_ = os.Remove(finalDestPath)
			return "", "", errors.New("uploaded file is corrupt or not a valid video stream")
		}
	}

	// Persist chunked videos up to 50 MB to durable database media storage so they survive Render redeployments
	if session.FileSize <= 50*1024*1024 {
		if sf, oErr := os.Open(finalDestPath); oErr == nil {
			if vData, rErr := io.ReadAll(sf); rErr == nil && len(vData) > 0 {
				_ = repository.SaveMediaBlob(session.TargetKey, session.ContentType, vData)
			}
			sf.Close()
		}
	}

	// If cloud S3 is configured, stream final file to S3 in background/mirror
	if utils.IsObjectStorageEnabled() {
		if sf, sErr := os.Open(finalDestPath); sErr == nil {
			_, _ = utils.UploadToObjectStorage(session.TargetKey, sf, totalWritten, session.ContentType)
			sf.Close()
		}
	}

	finalURL = "/uploads/" + session.TargetKey
	return session.TargetKey, finalURL, nil
}

// AbortChunkedUpload cancels an active upload session and deletes temporary chunks.
func AbortChunkedUpload(userID int, uploadID string) error {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	session, err := loadSession(uploadID)
	if err == nil {
		if session.UserID != userID {
			return errors.New("unauthorized: session belongs to another user")
		}
		if session.S3UploadID != "" {
			_ = utils.AbortS3MultipartUpload(session.TargetKey, session.S3UploadID)
		}
	}

	sessionDir := getSessionDir(uploadID)
	return os.RemoveAll(sessionDir)
}

// CleanupStaleUploads deletes unfinished chunks and sessions older than the specified maxAge.
func CleanupStaleUploads(maxAge time.Duration) int {
	tempDir := getTempChunksDir()
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return 0
	}

	cleaned := 0
	threshold := time.Now().Add(-maxAge)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessionDir := filepath.Join(tempDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(threshold) {
			// Check if S3 upload needs aborting
			if s, err := loadSession(entry.Name()); err == nil && s.S3UploadID != "" {
				_ = utils.AbortS3MultipartUpload(s.TargetKey, s.S3UploadID)
			}
			if err := os.RemoveAll(sessionDir); err == nil {
				cleaned++
			}
		}
	}

	if cleaned > 0 {
		log.Printf("[Upload] Cleaned up %d stale chunked upload sessions", cleaned)
	}
	return cleaned
}
