package handlers

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"convention-management-system/database"
	"convention-management-system/repository"
	"convention-management-system/sessions"
	"convention-management-system/utils"
)

func setupProfilePhotoTestDB(t *testing.T) func() {
	t.Helper()
	origWd, _ := os.Getwd()
	if strings.HasSuffix(origWd, "handlers") {
		_ = os.Chdir("..")
	}

	origDB := database.DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "profile_photo_test.db")

	testDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	database.DB = testDB

	if err := database.MigrateSQLite(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	utils.SetMediaStorageHooks(repository.SaveMediaBlob, repository.DeleteMediaBlob)

	// Seed user
	_, err = testDB.Exec(`
		INSERT INTO users (id, full_name, username, email, password_hash)
		VALUES (1, 'Samuel Prophet', 'samuel_p', 'samuel@example.com', 'hash');
	`)
	if err != nil {
		t.Fatalf("failed inserting test user: %v", err)
	}

	return func() {
		_ = os.Chdir(origWd)
		testDB.Close()
		database.DB = origDB
	}
}

func createTestJPEGBytes(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Draw colored rectangle
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

func TestUploadProfilePhoto_Base64FacialLiveness(t *testing.T) {
	cleanup := setupProfilePhotoTestDB(t)
	defer cleanup()

	// 1. Generate valid 200x200 JPEG base64
	jpegBytes := createTestJPEGBytes(200, 200)
	b64String := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpegBytes)

	reqPayload := map[string]any{
		"photo_base64": b64String,
		"consent":      true,
	}
	payloadBytes, _ := json.Marshal(reqPayload)

	req := httptest.NewRequest(http.MethodPost, "/profile/upload-photo", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	session, _ := sessions.Store.Get(req, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(req, httptest.NewRecorder())

	rr := httptest.NewRecorder()
	UploadProfilePhoto(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for base64 photo upload, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed decoding json response: %v", err)
	}
	if resp["success"] != true {
		t.Errorf("expected success=true, got %v", resp)
	}

	savedFile := resp["profile_photo"].(string)
	if !strings.HasPrefix(savedFile, "profile_1_") {
		t.Errorf("expected saved file to start with profile_1_, got %s", savedFile)
	}

	// Verify file was written to disk
	fullPath := filepath.Join("uploads/profiles", savedFile)
	if _, err := os.Stat(fullPath); err != nil {
		t.Errorf("expected file %s to exist on disk: %v", fullPath, err)
	}
	_ = os.Remove(fullPath)

	// Verify photo was durably persisted into database media_storage
	blobData, ctype, err := repository.GetMediaBlob("profiles/" + savedFile)
	if err != nil || len(blobData) == 0 {
		t.Errorf("expected profile photo to be durably persisted in database: %v", err)
	}
	if !strings.Contains(ctype, "image/") {
		t.Errorf("expected image content type in media_storage, got %s", ctype)
	}
}

func TestUploadProfilePhoto_MultipartFallback(t *testing.T) {
	cleanup := setupProfilePhotoTestDB(t)
	defer cleanup()

	jpegBytes := createTestJPEGBytes(150, 150)

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, err := w.CreateFormFile("profile_photo", "test_avatar.jpg")
	if err != nil {
		t.Fatalf("failed creating form file: %v", err)
	}
	_, _ = part.Write(jpegBytes)
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/profile/upload-photo", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	session, _ := sessions.Store.Get(req, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(req, httptest.NewRecorder())

	rr := httptest.NewRecorder()
	UploadProfilePhoto(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for multipart photo upload, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true, got %v", resp)
	}

	savedFile, ok := resp["profile_photo"].(string)
	if !ok || savedFile == "" {
		t.Fatalf("expected profile_photo filename in response, got %v", resp["profile_photo"])
	}

	// Verify multipart upload was durably persisted into database media_storage
	blobData, ctype, err := repository.GetMediaBlob("profiles/" + savedFile)
	if err != nil || len(blobData) == 0 {
		t.Errorf("expected multipart profile photo to be durably persisted in database: %v", err)
	}
	if !strings.Contains(ctype, "image/") {
		t.Errorf("expected image content type in media_storage, got %s", ctype)
	}
	_ = os.Remove(filepath.Join("uploads/profiles", savedFile))
}

