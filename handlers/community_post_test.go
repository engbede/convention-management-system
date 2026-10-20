package handlers

import (
	"bytes"
	"encoding/json"
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

// Helper to create a multipart POST request with session
func makeMultipartPostReq(t *testing.T, userID int, fields map[string]string, fileField, fileName string, fileBytes []byte, fileContentType string, isAJAX bool) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatalf("Failed to write field %s: %v", k, err)
		}
	}

	if fileField != "" && len(fileBytes) > 0 {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="`+fileField+`"; filename="`+fileName+`"`)
		if fileContentType != "" {
			h.Set("Content-Type", fileContentType)
		}
		part, err := writer.CreatePart(h)
		if err != nil {
			t.Fatalf("Failed to create part: %v", err)
		}
		if _, err := part.Write(fileBytes); err != nil {
			t.Fatalf("Failed to write file data: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/community/post", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if isAJAX {
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}

	if userID > 0 {
		initReq := httptest.NewRequest(http.MethodPost, "/community/post", nil)
		saveRec := httptest.NewRecorder()
		session, _ := sessions.Store.Get(initReq, "youth-community")
		session.Values["user_id"] = userID
		_ = session.Save(initReq, saveRec)

		for _, cookie := range saveRec.Result().Cookies() {
			req.AddCookie(cookie)
		}
	}

	return req, httptest.NewRecorder()
}

func sampleMP4Bytes() []byte {
	return []byte{
		0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p',
		'm', 'p', '4', '2', 0x00, 0x00, 0x00, 0x00,
		'm', 'p', '4', '2', 'i', 's', 'o', 'm',
		0x00, 0x00, 0x00, 0x08, 'm', 'd', 'a', 't',
		0x01, 0x02, 0x03, 0x04,
	}
}

func sampleWebMBytes() []byte {
	return []byte{
		0x1A, 0x45, 0xDF, 0xA3, 0x9F, 0x42, 0x86, 0x81,
		0x01, 0x42, 0xF7, 0x81, 0x01, 0x42, 0xF2, 0x81,
		0x04, 0x42, 0xF3, 0x81, 0x08, 0x42, 0x82, 0x84,
		'w', 'e', 'b', 'm',
	}
}

func samplePNGBytes() []byte {
	return []byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0x00, 0x00, 0x00, 0x0D, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0A, 'I', 'D', 'A',
		'T', 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
		0x00, 0x00, 0x00, 'I', 'E', 'N', 'D', 0xae,
		0x42, 0x60, 0x82,
	}
}

func TestCreateCommunityPost_TextOnly(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	fields := map[string]string{
		"content":    "Encouraging word for the youth!",
		"visibility": "public",
	}

	req, rr := makeMultipartPostReq(t, 1, fields, "", "", nil, "", true)
	CreateCommunityPost(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	if res["success"] != true {
		t.Errorf("Expected success: true, got %v", res)
	}

	// Verify post in database
	posts, err := services.GetCommunityFeed()
	if err != nil {
		t.Fatalf("Failed to get feed: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("Expected 1 post, got %d", len(posts))
	}
	if posts[0].Content != "Encouraging word for the youth!" {
		t.Errorf("Unexpected content: %s", posts[0].Content)
	}
	if posts[0].Video != "" {
		t.Errorf("Expected empty video, got %s", posts[0].Video)
	}
}

func TestCreateCommunityPost_VideoOnly_AJAX(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	fields := map[string]string{
		"content":    "",
		"visibility": "public",
	}

	req, rr := makeMultipartPostReq(t, 1, fields, "video", "sermon.mp4", sampleMP4Bytes(), "video/mp4", true)
	CreateCommunityPost(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	if res["success"] != true {
		t.Errorf("Expected success: true, got %v", res)
	}

	// Verify feed contains post with video
	posts, err := services.GetCommunityFeed()
	if err != nil {
		t.Fatalf("Failed to get feed: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("Expected 1 post, got %d", len(posts))
	}
	if !strings.HasPrefix(posts[0].Video, "posts/") || !strings.HasSuffix(posts[0].Video, ".mp4") {
		t.Errorf("Expected video reference posts/*.mp4, got %q", posts[0].Video)
	}
	if !strings.HasPrefix(posts[0].VideoURL(), "/uploads/posts/") {
		t.Errorf("Expected VideoURL() to resolve to /uploads/posts/*, got %q", posts[0].VideoURL())
	}

	// Verify the actual file was durably stored on disk
	storedPath := filepath.Join(utils.GetUploadBaseDir(), posts[0].Video)
	if _, err := os.Stat(storedPath); os.IsNotExist(err) {
		t.Errorf("Video file was not saved to storage at %s", storedPath)
	}
	defer utils.DeleteFile("uploads/" + posts[0].Video)
}

func TestCreateCommunityPost_VideoWithText_StandardForm(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	fields := map[string]string{
		"content":    "Convention highlights video clip",
		"visibility": "members",
	}

	req, rr := makeMultipartPostReq(t, 2, fields, "video", "highlights.mp4", sampleMP4Bytes(), "video/mp4", false)
	CreateCommunityPost(rr, req)

	// Standard form should redirect with 303 See Other
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("Expected 303 See Other, got %d: %s", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); loc != "/community" {
		t.Errorf("Expected redirect to /community, got %q", loc)
	}

	posts, err := services.GetCommunityFeed()
	if err != nil {
		t.Fatalf("Failed to get feed: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("Expected 1 post, got %d", len(posts))
	}
	if posts[0].Content != "Convention highlights video clip" {
		t.Errorf("Unexpected content: %s", posts[0].Content)
	}
	if posts[0].Visibility != "members" {
		t.Errorf("Expected visibility members, got %s", posts[0].Visibility)
	}
	if !strings.HasPrefix(posts[0].Video, "posts/") {
		t.Errorf("Expected video saved, got %q", posts[0].Video)
	}
	defer utils.DeleteFile("uploads/" + posts[0].Video)
}

func TestCreateCommunityPost_WebMVideo(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	fields := map[string]string{
		"content": "Browser recorded testimony",
	}

	req, rr := makeMultipartPostReq(t, 1, fields, "video", "recording.webm", sampleWebMBytes(), "video/webm", true)
	CreateCommunityPost(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	posts, err := services.GetCommunityFeed()
	if err != nil {
		t.Fatalf("Failed to get feed: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("Expected 1 post, got %d", len(posts))
	}
	if !strings.HasSuffix(posts[0].Video, ".webm") {
		t.Errorf("Expected .webm video, got %q", posts[0].Video)
	}
	defer utils.DeleteFile("uploads/" + posts[0].Video)
}

func TestCreateCommunityPost_ImageOnly(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	fields := map[string]string{
		"content": "",
	}

	req, rr := makeMultipartPostReq(t, 1, fields, "image", "photo.png", samplePNGBytes(), "image/png", true)
	CreateCommunityPost(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	posts, err := services.GetCommunityFeed()
	if err != nil {
		t.Fatalf("Failed to get feed: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("Expected 1 post, got %d", len(posts))
	}
	if !strings.HasPrefix(posts[0].Image, "posts/") {
		t.Errorf("Expected image to be saved, got %q", posts[0].Image)
	}
	defer utils.DeleteFile("uploads/" + posts[0].Image)
}

func TestCreateCommunityPost_InvalidVideoFormat(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	fields := map[string]string{
		"content": "Trying to upload non-video file",
	}

	// Text file disguised as mp4
	req, rr := makeMultipartPostReq(t, 1, fields, "video", "exploit.mp4", []byte("plain text script"), "text/plain", true)
	CreateCommunityPost(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request, got %d: %s", rr.Code, rr.Body.String())
	}

	var res map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &res)
	if res["success"] == true {
		t.Errorf("Expected success: false")
	}

	// Verify no post was inserted
	posts, err := services.GetCommunityFeed()
	if err != nil {
		t.Fatalf("Failed to get feed: %v", err)
	}
	if len(posts) != 0 {
		t.Errorf("Expected 0 posts in DB, got %d", len(posts))
	}
}

func TestCreateCommunityPost_EmptyPost(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	fields := map[string]string{
		"content": "   ",
	}

	req, rr := makeMultipartPostReq(t, 1, fields, "", "", nil, "", true)
	CreateCommunityPost(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for empty post, got %d", rr.Code)
	}
}

func TestCreateCommunityPost_Unauthenticated(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	fields := map[string]string{
		"content": "No session post",
	}

	req, rr := makeMultipartPostReq(t, 0, fields, "", "", nil, "", true)
	CreateCommunityPost(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 Unauthorized, got %d", rr.Code)
	}
}

func TestCreateCommunityPost_DatabaseFailure_CleansUpOrphanedVideo(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Simulate database failure by dropping the posts table
	if _, err := database.DB.Exec("DROP TABLE posts;"); err != nil {
		t.Fatalf("Failed to drop posts table: %v", err)
	}

	// Snapshot posts directory before the failed request
	postsDir := filepath.Join(utils.GetUploadBaseDir(), "posts")
	beforeFiles := make(map[string]bool)
	if entries, err := os.ReadDir(postsDir); err == nil {
		for _, e := range entries {
			beforeFiles[e.Name()] = true
		}
	}

	fields := map[string]string{
		"content": "Post with video during DB failure",
	}

	req, rr := makeMultipartPostReq(t, 1, fields, "video", "orphan_test.mp4", sampleMP4Bytes(), "video/mp4", true)
	CreateCommunityPost(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("Expected 500 Internal Server Error, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify no newly created video file was orphaned on disk
	if entries, err := os.ReadDir(postsDir); err == nil {
		for _, entry := range entries {
			if !beforeFiles[entry.Name()] && strings.HasSuffix(entry.Name(), ".mp4") {
				t.Errorf("Found orphaned video file %s after DB error", entry.Name())
				_ = os.Remove(filepath.Join(postsDir, entry.Name()))
			}
		}
	}
}

func TestPostVideoURL_Method(t *testing.T) {
	p := models.Post{Video: "posts/abc12345.mp4"}
	if p.VideoURL() != "/uploads/posts/abc12345.mp4" {
		t.Errorf("Expected /uploads/posts/abc12345.mp4, got %s", p.VideoURL())
	}

	pExternal := models.Post{Video: "https://storage.googleapis.com/videos/clip.mp4"}
	if pExternal.VideoURL() != "https://storage.googleapis.com/videos/clip.mp4" {
		t.Errorf("Expected external URL, got %s", pExternal.VideoURL())
	}
}

func TestCommunityFeed_VideoPostRendering(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Post a video
	fields := map[string]string{
		"content":    "Gospel choir convention rehearsal video",
		"visibility": "public",
	}
	req, rr := makeMultipartPostReq(t, 1, fields, "video", "choir.mp4", sampleMP4Bytes(), "video/mp4", true)
	CreateCommunityPost(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Failed to create video post: %d", rr.Code)
	}
	posts, _ := services.GetCommunityFeed()
	if len(posts) > 0 && posts[0].Video != "" {
		defer utils.DeleteFile("uploads/" + posts[0].Video)
	}

	// 2. Request the community page
	feedReq := httptest.NewRequest(http.MethodGet, "/community", nil)
	feedRR := httptest.NewRecorder()
	initReq := httptest.NewRequest(http.MethodGet, "/community", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		feedReq.AddCookie(cookie)
	}

	Community(feedRR, feedReq)
	if feedRR.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from Community feed, got %d", feedRR.Code)
	}

	body := feedRR.Body.String()

	// Verify post content and video elements are in the rendered HTML
	if !strings.Contains(body, "Gospel choir convention rehearsal video") {
		t.Errorf("Feed HTML does not contain post content")
	}
	if !strings.Contains(body, "<video") {
		t.Errorf("Feed HTML does not contain <video tag")
	}
	if !strings.Contains(body, "post-video-container") {
		t.Errorf("Feed HTML does not contain post-video-container")
	}
	if !strings.Contains(body, `/uploads/posts/`) {
		t.Errorf("Feed HTML does not contain /uploads/posts/ source path")
	}
}

func TestCommunityFeed_PersistenceAcrossSession(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. User 1 posts a video
	fields := map[string]string{
		"content": "Youth fellowship highlight",
	}
	req, rr := makeMultipartPostReq(t, 1, fields, "video", "fellowship.mp4", sampleMP4Bytes(), "video/mp4", true)
	CreateCommunityPost(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Failed to create video post: %d", rr.Code)
	}

	// 2. Fetch feed as user 1
	posts1, err := services.GetCommunityFeed()
	if err != nil || len(posts1) != 1 {
		t.Fatalf("Expected 1 post for user 1, got %v", err)
	}
	videoPath := posts1[0].Video
	defer utils.DeleteFile("uploads/" + videoPath)

	// 3. User 2 logs in and views feed (simulating logout/login)
	feedReq2 := httptest.NewRequest(http.MethodGet, "/community", nil)
	feedRR2 := httptest.NewRecorder()
	initReq2 := httptest.NewRequest(http.MethodGet, "/community", nil)
	saveRec2 := httptest.NewRecorder()
	session2, _ := sessions.Store.Get(initReq2, "youth-community")
	session2.Values["user_id"] = 2
	_ = session2.Save(initReq2, saveRec2)
	for _, cookie := range saveRec2.Result().Cookies() {
		feedReq2.AddCookie(cookie)
	}

	Community(feedRR2, feedReq2)
	if feedRR2.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from Community feed for user 2, got %d", feedRR2.Code)
	}

	body2 := feedRR2.Body.String()
	if !strings.Contains(body2, "Youth fellowship highlight") {
		t.Errorf("User 2 cannot see the post content")
	}
	if !strings.Contains(body2, videoPath) {
		t.Errorf("User 2 feed HTML does not contain video reference %s", videoPath)
	}
}

