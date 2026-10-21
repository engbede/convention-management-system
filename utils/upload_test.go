package utils_test

import (
	"bytes"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"convention-management-system/utils"
)

func createMultipartFile(t *testing.T, filename string, content []byte, contentType string) (multipart.File, *multipart.FileHeader) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}

	part, err := writer.CreatePart(h)
	if err != nil {
		t.Fatalf("Failed to create multipart part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("Failed to write content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	reader := multipart.NewReader(&body, writer.Boundary())
	form, err := reader.ReadForm(int64(body.Len()))
	if err != nil {
		t.Fatalf("Failed to read form: %v", err)
	}

	files := form.File["file"]
	if len(files) == 0 {
		t.Fatalf("No file parsed in form")
	}

	file, err := files[0].Open()
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}

	return file, files[0]
}

func TestSaveUploadedVideo_Success(t *testing.T) {
	destDir := t.TempDir()

	mp4Data := utils.GenerateValidTestMP4()

	file, header := createMultipartFile(t, "sample.mp4", mp4Data, "video/mp4")
	savedName, err := utils.SaveUploadedVideo(file, header, destDir)
	if err != nil {
		t.Fatalf("Expected successful video save, got err: %v", err)
	}
	if !strings.HasSuffix(savedName, ".mp4") {
		t.Errorf("Expected saved filename to have .mp4 suffix, got %s", savedName)
	}

	savedPath := filepath.Join(destDir, savedName)
	fi, err := os.Stat(savedPath)
	if err != nil {
		t.Fatalf("Saved file does not exist on disk: %v", err)
	}
	if fi.Size() != int64(len(mp4Data)) {
		t.Errorf("Expected file size %d, got %d", len(mp4Data), fi.Size())
	}
}

func TestValidateVideoIntegrity(t *testing.T) {
	t.Run("Valid MP4 container with moov and ftyp", func(t *testing.T) {
		mp4 := utils.GenerateValidTestMP4()
		r := bytes.NewReader(mp4)
		meta, err := utils.ValidateVideoIntegrity(r, int64(len(mp4)), "clip.mp4")
		if err != nil {
			t.Fatalf("Expected valid MP4 to pass integrity check, got err: %v", err)
		}
		if meta.Container != "mp4" {
			t.Errorf("Expected container mp4, got %s", meta.Container)
		}
		if !meta.IsWebPlayable {
			t.Errorf("Expected avc1 MP4 to be web playable")
		}
	})

	t.Run("Truncated MP4 missing moov atom fails", func(t *testing.T) {
		// Mock 36-byte file with only ftyp and incomplete mdat (missing moov atom)
		brokenBytes := []byte{
			0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p',
			'm', 'p', '4', '2', 0x00, 0x00, 0x00, 0x00,
			'm', 'p', '4', '2', 'i', 's', 'o', 'm',
			0x00, 0x00, 0x00, 0x08, 'm', 'd', 'a', 't',
			0x01, 0x02, 0x03, 0x04,
		}
		r := bytes.NewReader(brokenBytes)
		_, err := utils.ValidateVideoIntegrity(r, int64(len(brokenBytes)), "broken.mp4")
		if err == nil || !strings.Contains(err.Error(), "too small") {
			t.Errorf("Expected too small error for 36-byte file, got: %v", err)
		}
	})

	t.Run("Larger MP4 without moov atom fails with missing moov error", func(t *testing.T) {
		// 500-byte file with ftyp but no moov atom
		data := make([]byte, 500)
		copy(data[0:8], []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p'})
		copy(data[8:24], []byte("isom\x00\x00\x02\x00isomiso2"))
		// mdat takes up remaining 476 bytes
		copy(data[24:32], []byte{0x00, 0x00, 0x01, 0xDC, 'm', 'd', 'a', 't'})

		r := bytes.NewReader(data)
		_, err := utils.ValidateVideoIntegrity(r, int64(len(data)), "recording.mp4")
		if err == nil || !strings.Contains(err.Error(), "missing movie header") {
			t.Errorf("Expected missing moov atom error, got: %v", err)
		}
	})
}

func TestSaveUploadedVideo_WebM(t *testing.T) {
	destDir := t.TempDir()

	// WebM standard EBML header signature: 1A 45 DF A3
	webmHeader := []byte{
		0x1A, 0x45, 0xDF, 0xA3, 0x9F, 0x42, 0x86, 0x81,
		0x01, 0x42, 0xF7, 0x81, 0x01, 0x42, 0xF2, 0x81,
		0x04, 0x42, 0xF3, 0x81, 0x08, 0x42, 0x82, 0x84,
		'w', 'e', 'b', 'm',
	}
	// Pad with dummy EBML payload to reach valid minimum size
	webmHeader = append(webmHeader, make([]byte, 120)...)

	file, header := createMultipartFile(t, "clip.webm", webmHeader, "video/webm")
	savedName, err := utils.SaveUploadedVideo(file, header, destDir)
	if err != nil {
		t.Fatalf("Expected successful webm save, got err: %v", err)
	}
	if !strings.HasSuffix(savedName, ".webm") {
		t.Errorf("Expected saved filename to have .webm suffix, got %s", savedName)
	}
}

func TestSaveUploadedVideo_ValidationErrors(t *testing.T) {
	destDir := t.TempDir()

	t.Run("Empty file", func(t *testing.T) {
		file, header := createMultipartFile(t, "empty.mp4", []byte{}, "video/mp4")
		_, err := utils.SaveUploadedVideo(file, header, destDir)
		if err == nil || !strings.Contains(err.Error(), "empty") {
			t.Errorf("Expected empty file error, got: %v", err)
		}
	})

	t.Run("Exceeds default max size limit", func(t *testing.T) {
		file, header := createMultipartFile(t, "huge.mp4", []byte("too big"), "video/mp4")
		header.Size = int64(10241 * 1024 * 1024) // Fake header size > 10GB (10240 MB)
		_, err := utils.SaveUploadedVideo(file, header, destDir)
		if err == nil || !strings.Contains(err.Error(), "must not exceed 10240 MB") {
			t.Errorf("Expected 10240 MB max size error, got: %v", err)
		}
	})

	t.Run("Respects custom MAX_VIDEO_SIZE_MB limit", func(t *testing.T) {
		t.Setenv("MAX_VIDEO_SIZE_MB", "100")
		file, header := createMultipartFile(t, "oversize.mp4", []byte("too big"), "video/mp4")
		header.Size = 101 * 1024 * 1024
		_, err := utils.SaveUploadedVideo(file, header, destDir)
		if err == nil || !strings.Contains(err.Error(), "must not exceed 100 MB") {
			t.Errorf("Expected 100 MB max size error, got: %v", err)
		}
	})

	t.Run("Unsupported file extension", func(t *testing.T) {
		file, header := createMultipartFile(t, "script.sh", []byte("#!/bin/bash\necho bad"), "text/plain")
		_, err := utils.SaveUploadedVideo(file, header, destDir)
		if err == nil || !strings.Contains(err.Error(), "only mp4, webm, mov, mkv and avi") {
			t.Errorf("Expected unsupported extension error, got: %v", err)
		}
	})

	t.Run("Spoofed extension with plain text content", func(t *testing.T) {
		file, header := createMultipartFile(t, "fake.mp4", []byte("This is plain text pretending to be video"), "text/plain")
		_, err := utils.SaveUploadedVideo(file, header, destDir)
		if err == nil || !strings.Contains(err.Error(), "not a valid video") {
			t.Errorf("Expected invalid video error for text content, got: %v", err)
		}
	})
}

func TestDeleteFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "sample.txt")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	if err := utils.DeleteFile(filePath); err != nil {
		t.Errorf("DeleteFile failed: %v", err)
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("File was not deleted")
	}

	// Deleting non-existent file should return nil
	if err := utils.DeleteFile(filePath); err != nil {
		t.Errorf("Expected nil when deleting non-existent file, got: %v", err)
	}
}

