package services_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"convention-management-system/services"
	"convention-management-system/utils"
)

func sampleMP4Bytes() []byte {
	return []byte{
		0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p',
		'm', 'p', '4', '2', 0x00, 0x00, 0x00, 0x00,
		'm', 'p', '4', '2', 'i', 's', 'o', 'm',
		0x00, 0x00, 0x00, 0x08, 'm', 'd', 'a', 't',
		0x01, 0x02, 0x03, 0x04,
	}
}

func TestInitiateChunkedUpload_Limits(t *testing.T) {
	// 1. Test 508.2 MB file (the previously failing user file)
	size508MB := int64(5082 * 1024 * 1024 / 10) // 532,886,323 bytes
	sess, err := services.InitiateChunkedUpload(1, "service_recording_508mb.mp4", size508MB, "video/mp4")
	if err != nil {
		t.Fatalf("Expected 508.2 MB video to be accepted under 10 GB limit, got: %v", err)
	}
	if sess.UploadID == "" || sess.TotalChunks <= 0 {
		t.Errorf("Invalid session returned: %+v", sess)
	}
	_ = services.AbortChunkedUpload(1, sess.UploadID)

	// 2. Test 1.5 GB representative large recording (4+ hour church programme)
	size1500MB := int64(1500 * 1024 * 1024)
	sess1500, err := services.InitiateChunkedUpload(1, "convention_4hours_1500mb.mp4", size1500MB, "video/mp4")
	if err != nil {
		t.Fatalf("Expected 1.5 GB video to be accepted, got: %v", err)
	}
	_ = services.AbortChunkedUpload(1, sess1500.UploadID)

	// 3. Test approaching 10 GB limit (9.8 GB)
	size9800MB := int64(9800 * 1024 * 1024)
	sess9800, err := services.InitiateChunkedUpload(1, "all_day_convention_9800mb.mp4", size9800MB, "video/mp4")
	if err != nil {
		t.Fatalf("Expected 9.8 GB video to be accepted under 10 GB policy, got: %v", err)
	}
	_ = services.AbortChunkedUpload(1, sess9800.UploadID)

	// 4. Test exceeding custom MAX_VIDEO_SIZE_MB
	t.Setenv("MAX_VIDEO_SIZE_MB", "500")
	_, err = services.InitiateChunkedUpload(1, "too_large.mp4", size508MB, "video/mp4")
	if err == nil || !strings.Contains(err.Error(), "exceeds maximum allowed limit") {
		t.Errorf("Expected 508 MB to be rejected when MAX_VIDEO_SIZE_MB=500, got: %v", err)
	}

	// 5. Test invalid format
	_, err = services.InitiateChunkedUpload(1, "script.sh", 1024, "text/plain")
	if err == nil || !strings.Contains(err.Error(), "unsupported video format") {
		t.Errorf("Expected unsupported format error, got: %v", err)
	}

	// 6. Test zero bytes
	_, err = services.InitiateChunkedUpload(1, "empty.mp4", 0, "video/mp4")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected empty file error, got: %v", err)
	}

	// 7. Test unauthorized user
	_, err = services.InitiateChunkedUpload(0, "video.mp4", 1024, "video/mp4")
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("Expected unauthorized error, got: %v", err)
	}
}

func TestChunkedUpload_FullCycle_SaveAssembleVerify(t *testing.T) {
	mp4Data := sampleMP4Bytes()
	totalSize := int64(len(mp4Data))

	// Chunk into 2 parts
	half := len(mp4Data) / 2
	part1 := mp4Data[:half]
	part2 := mp4Data[half:]

	t.Setenv("CHUNK_SIZE_BYTES", fmt.Sprintf("%d", half))

	// Initiate
	session, err := services.InitiateChunkedUpload(42, "sunday_service.mp4", totalSize, "video/mp4")
	if err != nil {
		t.Fatalf("InitiateChunkedUpload failed: %v", err)
	}
	defer services.AbortChunkedUpload(42, session.UploadID)

	// Save Chunk 0
	err = services.SaveChunk(42, session.UploadID, 0, bytes.NewReader(part1), int64(len(part1)))
	if err != nil {
		t.Fatalf("SaveChunk 0 failed: %v", err)
	}

	// Verify Chunk 0 is tracked in uploaded chunks
	uploaded, err := services.GetUploadedChunks(42, session.UploadID)
	if err != nil || len(uploaded) != 1 || uploaded[0] != 0 {
		t.Fatalf("Expected [0] uploaded, got %v (err: %v)", uploaded, err)
	}

	// Save Chunk 1
	err = services.SaveChunk(42, session.UploadID, 1, bytes.NewReader(part2), int64(len(part2)))
	if err != nil {
		t.Fatalf("SaveChunk 1 failed: %v", err)
	}

	// Verify both chunks tracked
	uploaded, err = services.GetUploadedChunks(42, session.UploadID)
	if err != nil || len(uploaded) != 2 {
		t.Fatalf("Expected 2 chunks uploaded, got %v", uploaded)
	}

	// Complete Upload
	targetKey, finalURL, err := services.CompleteChunkedUpload(42, session.UploadID)
	if err != nil {
		t.Fatalf("CompleteChunkedUpload failed: %v", err)
	}

	if targetKey == "" || !strings.HasPrefix(targetKey, "posts/") {
		t.Errorf("Expected valid targetKey, got: %s", targetKey)
	}
	if finalURL == "" || !strings.Contains(finalURL, targetKey) {
		t.Errorf("Expected valid finalURL, got: %s", finalURL)
	}

	// Verify assembled file on disk
	assembledPath := filepath.Join(utils.GetUploadBaseDir(), targetKey)
	data, err := os.ReadFile(assembledPath)
	if err != nil {
		t.Fatalf("Failed reading assembled file: %v", err)
	}
	if !bytes.Equal(data, mp4Data) {
		t.Errorf("Assembled content mismatch: expected %d bytes, got %d bytes", len(mp4Data), len(data))
	}
	defer os.Remove(assembledPath)
}

func TestChunkedUpload_OwnershipAndSecurity(t *testing.T) {
	session, err := services.InitiateChunkedUpload(100, "privatesermon.mp4", 1024, "video/mp4")
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}
	defer services.AbortChunkedUpload(100, session.UploadID)

	// User 999 attempts to upload chunk for user 100's session
	err = services.SaveChunk(999, session.UploadID, 0, bytes.NewReader([]byte("hacked")), 6)
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("Expected unauthorized error for cross-user chunk save, got: %v", err)
	}

	// User 999 attempts to complete user 100's session
	_, _, err = services.CompleteChunkedUpload(999, session.UploadID)
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("Expected unauthorized error for cross-user completion, got: %v", err)
	}

	// User 999 attempts to abort user 100's session
	err = services.AbortChunkedUpload(999, session.UploadID)
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("Expected unauthorized error for cross-user abort, got: %v", err)
	}
}

func TestCleanupStaleUploads(t *testing.T) {
	session, err := services.InitiateChunkedUpload(5, "stale_video.mp4", 2048, "video/mp4")
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}

	// 0 duration maxAge cleans up immediately
	cleaned := services.CleanupStaleUploads(-1 * time.Second)
	if cleaned < 1 {
		t.Errorf("Expected at least 1 session cleaned up, got %d", cleaned)
	}

	// Confirm session is gone
	_, err = services.GetUploadedChunks(5, session.UploadID)
	if err == nil {
		t.Errorf("Expected session to be cleaned up, but was still found")
	}
}
