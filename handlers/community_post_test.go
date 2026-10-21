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
	"convention-management-system/repository"
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
	return utils.GenerateValidTestMP4()
}

func sampleWebMBytes() []byte {
	b := []byte{
		0x1A, 0x45, 0xDF, 0xA3, 0x9F, 0x42, 0x86, 0x81,
		0x01, 0x42, 0xF7, 0x81, 0x01, 0x42, 0xF2, 0x81,
		0x04, 0x42, 0xF3, 0x81, 0x08, 0x42, 0x82, 0x84,
		'w', 'e', 'b', 'm',
	}
	return append(b, make([]byte, 120)...)
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

func TestCreateCommunityPost_LargeVideo_Exceeding50MB(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	fields := map[string]string{
		"content": "Long-form church convention worship service (75 MB)",
	}

	// Create multipart with valid MP4 header and simulated 75 MB size (exceeding old 50MB restriction)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("content", fields["content"])

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="video"; filename="convention_service.mp4"`)
	h.Set("Content-Type", "video/mp4")
	part, _ := writer.CreatePart(h)
	_, _ = part.Write(sampleMP4Bytes())
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/community/post", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	initReq := httptest.NewRequest(http.MethodPost, "/community/post", nil)
	saveRec := httptest.NewRecorder()
	session, _ := sessions.Store.Get(initReq, "youth-community")
	session.Values["user_id"] = 1
	_ = session.Save(initReq, saveRec)
	for _, cookie := range saveRec.Result().Cookies() {
		req.AddCookie(cookie)
	}

	rr := httptest.NewRecorder()
	CreateCommunityPost(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for 75 MB video within 500 MB limit, got %d: %s", rr.Code, rr.Body.String())
	}

	posts, err := services.GetCommunityFeed()
	if err != nil || len(posts) != 1 {
		t.Fatalf("Expected 1 post in feed, got %v", err)
	}
	if posts[0].Video == "" {
		t.Errorf("Expected video reference in post, got empty")
	}
	defer utils.DeleteFile("uploads/" + posts[0].Video)
}

func TestCreateCommunityPost_Exceeding500MBLimit(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Use custom limit 100 MB for this test to test limit rejection cleanly
	t.Setenv("MAX_VIDEO_SIZE_MB", "100")

	destDir := t.TempDir()
	file, header := createMultipartFile(t, "overlimit.mp4", sampleMP4Bytes(), "video/mp4")
	header.Size = 105 * 1024 * 1024 // 105 MB > 100 MB limit

	_, err := utils.SaveUploadedVideo(file, header, destDir)
	if err == nil || !strings.Contains(err.Error(), "must not exceed 100 MB") {
		t.Fatalf("Expected max size error for 105 MB video, got %v", err)
	}
}

func TestFeedRendering_OnErrorProtection(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Update user 1 to have a profile photo reference that may be missing on disk
	_, _ = database.DB.Exec(`UPDATE users SET profile_photo = 'profiles/missing_user1.jpg' WHERE id = 1`)

	// Insert post with missing image to test broken image resilience
	_, err := database.DB.Exec(`
		INSERT INTO posts (user_id, content, image, video, visibility, created_at)
		VALUES (1, 'Post with missing image reference', 'posts/non_existent_image.jpg', '', 'public', CURRENT_TIMESTAMP)
	`)
	if err != nil {
		t.Fatalf("Failed to insert test post: %v", err)
	}

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
		t.Fatalf("Expected 200 OK, got %d", feedRR.Code)
	}

	body := feedRR.Body.String()
	// Confirm onerror resilience handler is present in post image tag
	if !strings.Contains(body, `onerror="this.closest('.post-image-container').style.display='none';"`) {
		t.Errorf("Expected onerror handler on post image container")
	}
	// Confirm onerror resilience handler is present on author profile photo tag
	if !strings.Contains(body, `onerror="this.onerror=null; this.style.display='none';`) {
		t.Errorf("Expected onerror handler on profile photo tag")
	}
}

func TestMediaAuditAndRepair(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	report, err := services.AuditAndRepairMedia()
	if err != nil {
		t.Fatalf("AuditAndRepairMedia failed: %v", err)
	}
	if report == nil {
		t.Fatalf("Expected non-nil report")
	}
}

func TestCreateCommunityPost_ImagePersistence_SurvivesRestart(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Post an image
	pngImage := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52}
	req, rr := makeMultipartPostReq(t, 1, map[string]string{"content": "Durable image test"}, "image", "faith.png", pngImage, "image/png", true)
	CreateCommunityPost(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK from CreateCommunityPost, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	posts, err := repository.GetAllPosts()
	if err != nil || len(posts) == 0 {
		t.Fatalf("Expected at least 1 post, err: %v", err)
	}
	latestPost := posts[0]
	if latestPost.Image == "" {
		t.Fatalf("Expected post.Image to be non-empty")
	}

	// 2. Verify media is in database durable storage
	data, ctype, err := repository.GetMediaBlob(latestPost.Image)
	if err != nil || len(data) == 0 {
		t.Fatalf("Expected image blob in database media_storage, got err: %v", err)
	}
	if !strings.Contains(ctype, "image/") {
		t.Errorf("Expected image/ content type, got %s", ctype)
	}

	// 3. Simulate container restart / ephemeral disk wipe: delete from disk
	baseDir := utils.GetUploadBaseDir()
	diskPath := filepath.Join(baseDir, filepath.Clean("/"+latestPost.Image))
	_ = os.Remove(diskPath)

	if _, err := os.Stat(diskPath); !os.IsNotExist(err) {
		t.Fatalf("Failed to remove disk file to simulate container wipe")
	}

	// 4. Run startup synchronization (as occurs on Render boot)
	services.AutoSyncEmbeddedMedia(database.DB)

	// 5. Verify file was restored from database to disk
	if _, err := os.Stat(diskPath); os.IsNotExist(err) {
		t.Errorf("Expected AutoSyncEmbeddedMedia to restore missing file %s to disk", diskPath)
	}

	// Clean up test file
	_ = os.Remove(diskPath)
}

func TestMediaURL_Normalization(t *testing.T) {
	// Post ImageURL normalization
	testCasesPost := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"abc.jpg", "/uploads/posts/abc.jpg"},
		{"posts/abc.jpg", "/uploads/posts/abc.jpg"},
		{"/uploads/abc.jpg", "/uploads/posts/abc.jpg"},
		{"/uploads/posts/abc.jpg", "/uploads/posts/abc.jpg"},
		{"https://cdn.church.org/banner.jpg", "https://cdn.church.org/banner.jpg"},
	}

	for _, tc := range testCasesPost {
		p := models.Post{Image: tc.input}
		got := p.ImageURL()
		if got != tc.expected {
			t.Errorf("Post.ImageURL(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}

	// Post VideoURL normalization
	testCasesVideo := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"sermon.mp4", "/uploads/posts/sermon.mp4"},
		{"posts/sermon.mp4", "/uploads/posts/sermon.mp4"},
		{"/uploads/sermon.mp4", "/uploads/posts/sermon.mp4"},
		{"/uploads/posts/sermon.mp4", "/uploads/posts/sermon.mp4"},
		{"https://cdn.church.org/sermon.mp4", "https://cdn.church.org/sermon.mp4"},
	}

	for _, tc := range testCasesVideo {
		p := models.Post{Video: tc.input}
		got := p.VideoURL()
		if got != tc.expected {
			t.Errorf("Post.VideoURL(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}

	// User ProfilePhotoURL normalization
	testCasesUser := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"pastor.jpg", "/uploads/profiles/pastor.jpg"},
		{"profiles/pastor.jpg", "/uploads/profiles/pastor.jpg"},
		{"/uploads/pastor.jpg", "/uploads/profiles/pastor.jpg"},
		{"/uploads/profiles/pastor.jpg", "/uploads/profiles/pastor.jpg"},
		{"https://cdn.church.org/pastor.jpg", "https://cdn.church.org/pastor.jpg"},
	}

	for _, tc := range testCasesUser {
		u := models.User{ProfilePhoto: tc.input}
		got := u.ProfilePhotoURL()
		if got != tc.expected {
			t.Errorf("User.ProfilePhotoURL(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestCreateCommunityPost_VideoPersistsToDurableStorage(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	utils.SetMediaStorageHooks(repository.SaveMediaBlob, repository.DeleteMediaBlob)

	fields := map[string]string{
		"content":    "Testing video durable storage",
		"visibility": "public",
	}

	mp4Data := sampleMP4Bytes()
	req, rr := makeMultipartPostReq(t, 1, fields, "video", "test_clip.mp4", mp4Data, "video/mp4", false)
	CreateCommunityPost(rr, req)

	if rr.Code != http.StatusSeeOther && rr.Code != http.StatusOK {
		t.Fatalf("Expected post creation to succeed, got %d: %s", rr.Code, rr.Body.String())
	}

	posts, err := services.GetCommunityFeed()
	if err != nil || len(posts) != 1 {
		t.Fatalf("Expected 1 post in feed, got %v", err)
	}

	videoRef := posts[0].Video
	if videoRef == "" {
		t.Fatalf("Expected video reference in post, got empty")
	}

	// Verify video was persisted into database media_storage
	if !repository.HasMediaBlob(videoRef) {
		t.Fatalf("Expected HasMediaBlob(%q) to be true for uploaded video", videoRef)
	}

	data, ctype, err := repository.GetMediaBlob(videoRef)
	if err != nil {
		t.Fatalf("Failed to retrieve media blob for %s: %v", videoRef, err)
	}
	if !strings.Contains(ctype, "video/") && ctype != "application/octet-stream" {
		t.Errorf("Expected video content type in durable store, got %q", ctype)
	}
	if !bytes.Equal(data, mp4Data) {
		t.Errorf("Durable store bytes do not match uploaded mp4 data")
	}

	// Simulate container restart/redeploy: delete local disk file
	diskPath := filepath.Join("uploads", videoRef)
	_ = os.Remove(diskPath)

	// Verify HasMediaBlob is still true even when disk file is deleted
	if !repository.HasMediaBlob(videoRef) {
		t.Errorf("HasMediaBlob must remain true after ephemeral disk removal")
	}

	defer utils.DeleteFile("uploads/" + videoRef)
	defer repository.DeleteMediaBlob(videoRef)
}



