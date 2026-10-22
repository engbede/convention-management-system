package utils_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"convention-management-system/utils"
)

func TestStorage_ConfigValidation(t *testing.T) {
	t.Run("Disabled when no env vars set", func(t *testing.T) {
		t.Setenv("S3_BUCKET", "")
		t.Setenv("AWS_S3_BUCKET", "")
		t.Setenv("S3_ACCESS_KEY", "")
		t.Setenv("S3_SECRET_KEY", "")

		if utils.IsObjectStorageEnabled() {
			t.Errorf("Expected IsObjectStorageEnabled to be false when env is unset")
		}
		if utils.GetS3Config() != nil {
			t.Errorf("Expected GetS3Config to return nil when env is unset")
		}
		if err := utils.ValidateStorageConfig(); err == nil {
			t.Errorf("Expected ValidateStorageConfig to return error when unconfigured")
		}
	})

	t.Run("Valid Cloudflare R2 config", func(t *testing.T) {
		t.Setenv("S3_BUCKET", "convention-media")
		t.Setenv("S3_ENDPOINT", "https://abc123def456.r2.cloudflarestorage.com")
		t.Setenv("S3_REGION", "auto")
		t.Setenv("S3_ACCESS_KEY", "test_access_key_12345")
		t.Setenv("S3_SECRET_KEY", "test_secret_key_67890abcdef")
		t.Setenv("S3_PUBLIC_URL", "https://media.convention.org")

		if !utils.IsObjectStorageEnabled() {
			t.Fatalf("Expected IsObjectStorageEnabled to be true")
		}
		if !utils.IsCloudflareR2() {
			t.Errorf("Expected IsCloudflareR2 to be true for r2.cloudflarestorage.com")
		}
		if err := utils.ValidateStorageConfig(); err != nil {
			t.Errorf("Expected ValidateStorageConfig to pass, got: %v", err)
		}

		cfg := utils.GetS3Config()
		if cfg.Bucket != "convention-media" {
			t.Errorf("Expected bucket convention-media, got %s", cfg.Bucket)
		}
		if cfg.Region != "auto" {
			t.Errorf("Expected region auto, got %s", cfg.Region)
		}
		if cfg.PublicURL != "https://media.convention.org" {
			t.Errorf("Expected public URL https://media.convention.org, got %s", cfg.PublicURL)
		}
	})

	t.Run("Invalid endpoint scheme rejects safely", func(t *testing.T) {
		t.Setenv("S3_BUCKET", "bucket")
		t.Setenv("S3_ENDPOINT", "ftp://invalid-server.com")
		t.Setenv("S3_ACCESS_KEY", "valid_key_123")
		t.Setenv("S3_SECRET_KEY", "valid_secret_12345")

		err := utils.ValidateStorageConfig()
		if err == nil || !strings.Contains(err.Error(), "scheme must be http or https") {
			t.Errorf("Expected invalid scheme error, got: %v", err)
		}
	})
}

func TestStorage_MockR2_UploadDownloadLifecycle(t *testing.T) {
	uploadedData := make(map[string][]byte)
	receivedHeaders := make(map[string]http.Header)

	// Mock S3/R2 HTTP Server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path
		receivedHeaders[key] = r.Header.Clone()

		// Verify that unsupported x-amz-acl header is NEVER sent to Cloudflare R2
		if acl := r.Header.Get("x-amz-acl"); acl != "" {
			t.Errorf("CRITICAL BUG: x-amz-acl header (%q) sent to R2 endpoint! Cloudflare R2 rejects this header.", acl)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("InvalidArgument: S3 ACLs are not supported by Cloudflare R2"))
			return
		}

		// Verify AWS SigV4 authorization header format
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") {
			t.Errorf("Expected AWS4-HMAC-SHA256 authorization header, got: %s", auth)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if !strings.Contains(auth, "Credential=") || !strings.Contains(auth, "Signature=") {
			t.Errorf("Malformed AWS SigV4 Authorization header: %s", auth)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch r.Method {
		case http.MethodPut:
			// Verify UNSIGNED-PAYLOAD is used for streaming without full memory buffering
			shaHeader := r.Header.Get("x-amz-content-sha256")
			if shaHeader != "UNSIGNED-PAYLOAD" && len(shaHeader) != 64 {
				t.Errorf("Expected UNSIGNED-PAYLOAD or 64-char sha256, got: %s", shaHeader)
			}

			data, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			uploadedData[key] = data
			w.Header().Set("ETag", `"mock-etag-12345678"`)
			w.WriteHeader(http.StatusOK)

		case http.MethodHead:
			data, exists := uploadedData[key]
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Length", string(rune(len(data))))
			w.WriteHeader(http.StatusOK)

		case http.MethodDelete:
			delete(uploadedData, key)
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer mockServer.Close()

	t.Setenv("S3_BUCKET", "test-bucket")
	t.Setenv("S3_ENDPOINT", mockServer.URL)
	t.Setenv("S3_REGION", "auto")
	t.Setenv("S3_ACCESS_KEY", "r2_access_key_test")
	t.Setenv("S3_SECRET_KEY", "r2_secret_key_test_secret_val")
	t.Setenv("S3_PUBLIC_URL", "")

	// 1. Upload sample object via streaming (reader)
	samplePayload := []byte("STREAMED-VIDEO-OR-IMAGE-PAYLOAD-FOR-CLOUDFLARE-R2-TESTING")
	targetKey := "posts/test_video.mp4"

	returnedURL, err := utils.UploadToObjectStorage(targetKey, bytes.NewReader(samplePayload), int64(len(samplePayload)), "video/mp4")
	if err != nil {
		t.Fatalf("UploadToObjectStorage failed: %v", err)
	}

	expectedPath := "/test-bucket/posts/test_video.mp4"
	if !strings.Contains(returnedURL, expectedPath) {
		t.Errorf("Expected URL to contain %q, got: %s", expectedPath, returnedURL)
	}

	// Verify server received exact content
	stored, ok := uploadedData[expectedPath]
	if !ok || !bytes.Equal(stored, samplePayload) {
		t.Errorf("Uploaded content mismatch in mock R2 server")
	}

	// 2. Check Object Exists via HEAD
	exists, size, err := utils.CheckObjectExists(targetKey)
	if err != nil || !exists {
		t.Errorf("CheckObjectExists failed: exists=%v, err=%v", exists, err)
	}
	_ = size

	// 3. Delete Object via DELETE
	if err := utils.DeleteFromObjectStorage(targetKey); err != nil {
		t.Errorf("DeleteFromObjectStorage failed: %v", err)
	}
	if _, stillThere := uploadedData[expectedPath]; stillThere {
		t.Errorf("Expected object to be deleted from mock R2 store")
	}
}

func TestStorage_PresignedURLs(t *testing.T) {
	t.Setenv("S3_BUCKET", "my-r2-bucket")
	t.Setenv("S3_ENDPOINT", "https://xyz999.r2.cloudflarestorage.com")
	t.Setenv("S3_REGION", "auto")
	t.Setenv("S3_ACCESS_KEY", "mock_key_abc")
	t.Setenv("S3_SECRET_KEY", "mock_secret_xyz12345")
	t.Setenv("S3_PUBLIC_URL", "")

	t.Run("GeneratePresignedGetURL produces valid SigV4 query params", func(t *testing.T) {
		signedURL, err := utils.GeneratePresignedGetURL("posts/sample_video.mp4", 15*time.Minute)
		if err != nil {
			t.Fatalf("GeneratePresignedGetURL failed: %v", err)
		}

		u, err := url.Parse(signedURL)
		if err != nil {
			t.Fatalf("Failed to parse signed URL: %v", err)
		}

		if u.Host != "xyz999.r2.cloudflarestorage.com" {
			t.Errorf("Expected host xyz999.r2.cloudflarestorage.com, got: %s", u.Host)
		}
		if u.Path != "/my-r2-bucket/posts/sample_video.mp4" {
			t.Errorf("Expected path /my-r2-bucket/posts/sample_video.mp4, got: %s", u.Path)
		}

		q := u.Query()
		if q.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" {
			t.Errorf("Missing or invalid X-Amz-Algorithm: %s", q.Get("X-Amz-Algorithm"))
		}
		if !strings.Contains(q.Get("X-Amz-Credential"), "mock_key_abc/") {
			t.Errorf("X-Amz-Credential does not contain access key: %s", q.Get("X-Amz-Credential"))
		}
		if q.Get("X-Amz-Signature") == "" {
			t.Errorf("Missing X-Amz-Signature in presigned URL")
		}
		if q.Get("X-Amz-Expires") != "900" {
			t.Errorf("Expected 900 seconds expiry for 15min, got: %s", q.Get("X-Amz-Expires"))
		}
	})

	t.Run("GeneratePresignedPutURL produces valid SigV4 PUT params", func(t *testing.T) {
		putURL, err := utils.GeneratePresignedPutURL("posts/user_camera_recording.mp4", "video/mp4", 10*time.Minute)
		if err != nil {
			t.Fatalf("GeneratePresignedPutURL failed: %v", err)
		}

		u, err := url.Parse(putURL)
		if err != nil {
			t.Fatalf("Failed to parse presigned PUT URL: %v", err)
		}

		q := u.Query()
		if q.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" {
			t.Errorf("Missing X-Amz-Algorithm in presigned PUT URL")
		}
		if q.Get("X-Amz-Signature") == "" {
			t.Errorf("Missing X-Amz-Signature in presigned PUT URL")
		}
		if q.Get("X-Amz-Expires") != "600" {
			t.Errorf("Expected 600 seconds expiry for 10min, got: %s", q.Get("X-Amz-Expires"))
		}
	})
}

