package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/services"
	"convention-management-system/sessions"
	"convention-management-system/utils"
)

func TestGetUploadConfigAPI(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/upload/config", nil)
	rr := httptest.NewRecorder()

	GetUploadConfigAPI(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var data map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &data); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if data["max_video_size_mb"].(float64) <= 0 {
		t.Errorf("Expected positive max_video_size_mb, got %v", data["max_video_size_mb"])
	}
	if data["chunk_size_bytes"].(float64) <= 0 {
		t.Errorf("Expected positive chunk_size_bytes, got %v", data["chunk_size_bytes"])
	}
}

func TestInitiateUploadAPI_Validation(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Unauthenticated request
	size508MB := int64(5082 * 1024 * 1024 / 10) // 508.2 MB = 532,886,323 bytes (> 500 MB limit of 524,288,000 bytes)
	body, _ := json.Marshal(map[string]any{
		"filename":     "church_service.mp4",
		"file_size":    size508MB,
		"content_type": "video/mp4",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/upload/initiate", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	InitiateUploadAPI(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized for anonymous user, got %d", rr.Code)
	}

	// 2. Authenticated request exceeding policy limit
	t.Setenv("MAX_VIDEO_SIZE_MB", "500")
	overlimitBody, _ := json.Marshal(map[string]any{
		"filename":     "church_service.mp4",
		"file_size":    size508MB,
		"content_type": "video/mp4",
	})
	reqAuth := httptest.NewRequest(http.MethodPost, "/api/upload/initiate", bytes.NewReader(overlimitBody))
	initReq := httptest.NewRequest(http.MethodPost, "/api/upload/initiate", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		reqAuth.AddCookie(cookie)
	}

	rrAuth := httptest.NewRecorder()
	InitiateUploadAPI(rrAuth, reqAuth)
	if rrAuth.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request when exceeding 500 MB limit, got %d", rrAuth.Code)
	}
}

func TestChunkedUploadHTTPFlow_AndCommunityPostCreation(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	mp4 := sampleMP4Bytes()
	half := len(mp4) / 2
	t.Setenv("CHUNK_SIZE_BYTES", fmt.Sprintf("%d", half))

	// Setup authenticated session
	initReq := httptest.NewRequest(http.MethodPost, "/community/post", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)
	sessionCookies := saveRec.Result().Cookies()

	// 1. Initiate Upload
	initBody, _ := json.Marshal(map[string]any{
		"filename":     "youth_convention_programme.mp4",
		"file_size":    len(mp4),
		"content_type": "video/mp4",
	})
	initReqHTTP := httptest.NewRequest(http.MethodPost, "/api/upload/initiate", bytes.NewReader(initBody))
	for _, c := range sessionCookies {
		initReqHTTP.AddCookie(c)
	}
	initRR := httptest.NewRecorder()
	InitiateUploadAPI(initRR, initReqHTTP)

	if initRR.Code != http.StatusOK {
		t.Fatalf("Initiate failed (%d): %s", initRR.Code, initRR.Body.String())
	}

	var initResp struct {
		Success     bool   `json:"success"`
		UploadID    string `json:"upload_id"`
		TotalChunks int    `json:"total_chunks"`
	}
	_ = json.Unmarshal(initRR.Body.Bytes(), &initResp)
	if !initResp.Success || initResp.UploadID == "" || initResp.TotalChunks != 2 {
		t.Fatalf("Unexpected initiate response: %+v", initResp)
	}

	// 2. Upload Chunk 0
	chunk0Req, chunk0RR := makeChunkPostReq(t, sessionCookies, initResp.UploadID, 0, mp4[:half])
	UploadChunkAPI(chunk0RR, chunk0Req)
	if chunk0RR.Code != http.StatusOK {
		t.Fatalf("Chunk 0 failed (%d): %s", chunk0RR.Code, chunk0RR.Body.String())
	}

	// Check status (resume verification)
	statusReq := httptest.NewRequest(http.MethodGet, "/api/upload/status?upload_id="+initResp.UploadID, nil)
	for _, c := range sessionCookies {
		statusReq.AddCookie(c)
	}
	statusRR := httptest.NewRecorder()
	GetUploadStatusAPI(statusRR, statusReq)
	if statusRR.Code != http.StatusOK || !strings.Contains(statusRR.Body.String(), "[0]") {
		t.Fatalf("Status expected chunk [0], got %d: %s", statusRR.Code, statusRR.Body.String())
	}

	// 3. Upload Chunk 1
	chunk1Req, chunk1RR := makeChunkPostReq(t, sessionCookies, initResp.UploadID, 1, mp4[half:])
	UploadChunkAPI(chunk1RR, chunk1Req)
	if chunk1RR.Code != http.StatusOK {
		t.Fatalf("Chunk 1 failed (%d): %s", chunk1RR.Code, chunk1RR.Body.String())
	}

	// 4. Complete Upload
	compBody, _ := json.Marshal(map[string]any{"upload_id": initResp.UploadID})
	compReq := httptest.NewRequest(http.MethodPost, "/api/upload/complete", bytes.NewReader(compBody))
	for _, c := range sessionCookies {
		compReq.AddCookie(c)
	}
	compRR := httptest.NewRecorder()
	CompleteUploadAPI(compRR, compReq)

	if compRR.Code != http.StatusOK {
		t.Fatalf("Complete failed (%d): %s", compRR.Code, compRR.Body.String())
	}

	var compResp struct {
		Success  bool   `json:"success"`
		VideoKey string `json:"video_key"`
		VideoURL string `json:"video_url"`
	}
	_ = json.Unmarshal(compRR.Body.Bytes(), &compResp)
	if !compResp.Success || compResp.VideoKey == "" {
		t.Fatalf("Expected valid VideoKey in complete response: %+v", compResp)
	}
	defer os.Remove(filepath.Join(utils.GetUploadBaseDir(), compResp.VideoKey))

	// 5. Create Community Post referencing the verified video_key
	var postBody bytes.Buffer
	writer := multipart.NewWriter(&postBody)
	_ = writer.WriteField("content", "Watch our 4-hour Youth Convention service!")
	_ = writer.WriteField("visibility", "public")
	_ = writer.WriteField("video_key", compResp.VideoKey)
	_ = writer.Close()

	postReq := httptest.NewRequest(http.MethodPost, "/community/post", &postBody)
	postReq.Header.Set("Content-Type", writer.FormDataContentType())
	postReq.Header.Set("Accept", "application/json")
	postReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	for _, c := range sessionCookies {
		postReq.AddCookie(c)
	}
	postRR := httptest.NewRecorder()

	CreateCommunityPost(postRR, postReq)
	if postRR.Code != http.StatusOK {
		t.Fatalf("Post creation failed (%d): %s", postRR.Code, postRR.Body.String())
	}

	// 6. Verify feed query includes post and resolves VideoURL
	posts, err := services.GetCommunityFeed()
	if err != nil || len(posts) == 0 {
		t.Fatalf("Expected post in community feed, err: %v", err)
	}
	found := false
	for _, p := range posts {
		if p.Video == compResp.VideoKey {
			found = true
			if !strings.HasPrefix(p.VideoURL(), "/uploads/") {
				t.Errorf("Expected VideoURL starting with /uploads/, got: %s", p.VideoURL())
			}
			break
		}
	}
	if !found {
		t.Errorf("Newly created post with video_key was not found in community feed")
	}
}

func TestCommunityPost_VideoKeyPathTraversalSecurity(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	initReq := httptest.NewRequest(http.MethodPost, "/community/post", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)

	// Attempt path traversal via video_key
	var postBody bytes.Buffer
	writer := multipart.NewWriter(&postBody)
	_ = writer.WriteField("content", "Testing traversal")
	_ = writer.WriteField("visibility", "public")
	_ = writer.WriteField("video_key", "posts/../../etc/passwd")
	_ = writer.Close()

	postReq := httptest.NewRequest(http.MethodPost, "/community/post", &postBody)
	postReq.Header.Set("Content-Type", writer.FormDataContentType())
	postReq.Header.Set("Accept", "application/json")
	postReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	for _, c := range saveRec.Result().Cookies() {
		postReq.AddCookie(c)
	}
	postRR := httptest.NewRecorder()

	CreateCommunityPost(postRR, postReq)
	// Post will succeed as text-only because video_key was safely rejected, or video won't be set to traversal path
	var post models.Post
	_ = database.DB.QueryRow("SELECT video FROM posts ORDER BY id DESC LIMIT 1").Scan(&post.Video)
	if post.Video != "" {
		t.Errorf("Path traversal video_key should not have been persisted, got: %s", post.Video)
	}
}

func makeChunkPostReq(t *testing.T, cookies []*http.Cookie, uploadID string, chunkIdx int, chunkBytes []byte) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("upload_id", uploadID)
	_ = writer.WriteField("chunk_index", fmt.Sprintf("%d", chunkIdx))

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="chunk"; filename="chunk.part"`)
	h.Set("Content-Type", "application/octet-stream")
	part, _ := writer.CreatePart(h)
	_, _ = part.Write(chunkBytes)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload/chunk", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	return req, httptest.NewRecorder()
}
