package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/repository"

	_ "github.com/mattn/go-sqlite3"
)

func setupDocTestDB(t *testing.T) func() {
	t.Helper()
	testDBPath := filepath.Join(t.TempDir(), "test_docs.db")
	db, err := sql.Open("sqlite3", testDBPath)
	if err != nil {
		t.Fatalf("Failed to open test db: %v", err)
	}
	origDB := database.DB
	database.DB = db

	if err := database.MigrateSQLite(); err != nil {
		t.Fatalf("Failed migrations: %v", err)
	}

	return func() {
		db.Close()
		database.DB = origDB
	}
}

func TestDownloadDocumentFile_SuccessAndErrors(t *testing.T) {
	cleanup := setupDocTestDB(t)
	defer cleanup()

	// 1. Create uploads test file
	uploadDir := "uploads/documents"
	_ = os.MkdirAll(uploadDir, 0755)
	defer os.RemoveAll("uploads")

	testContent := []byte("%PDF-1.4 sample convention minutes")
	sampleFile := filepath.Join(uploadDir, "convention_minutes_2026.pdf")
	if err := os.WriteFile(sampleFile, testContent, 0644); err != nil {
		t.Fatalf("Failed creating test file: %v", err)
	}

	// 2. Insert test document (category: Minutes)
	doc := models.Document{
		Title:       "2026 Diocesan Council Minutes",
		Category:    "Minutes",
		Convention:  "2026 Convention",
		Year:        2026,
		Description: "Official meeting minutes",
		CreatedBy:   "Secretary",
	}
	if err := repository.CreateDocument(doc); err != nil {
		t.Fatalf("Failed creating document: %v", err)
	}

	// Retrieve created doc ID
	docs, err := repository.GetAllDocuments()
	if err != nil || len(docs) == 0 {
		t.Fatalf("Failed getting created document: %v", err)
	}
	docID := docs[0].ID

	// Insert document_file
	docFile := models.DocumentFile{
		DocumentID: docID,
		FileName:   "convention_minutes_2026.pdf",
		FilePath:   sampleFile,
		FileType:   "application/pdf",
		UploadedAt: time.Now(),
	}
	if err := repository.SaveDocumentFile(docFile); err != nil {
		t.Fatalf("Failed saving document file: %v", err)
	}

	files, err := repository.GetDocumentFiles(docID)
	if err != nil || len(files) == 0 {
		t.Fatalf("Failed getting document files: %v", err)
	}
	fileRecordID := files[0].ID

	// Test A: Unauthorized access to restricted document (Minutes) by anonymous user
	reqAnon := httptest.NewRequest(http.MethodGet, "/documents/download?id="+strconv.Itoa(fileRecordID), nil)
	wAnon := httptest.NewRecorder()
	DownloadDocumentFile(wAnon, reqAnon)
	if wAnon.Code != http.StatusSeeOther {
		t.Errorf("Expected redirect 303 for anonymous download of Minutes, got %d", wAnon.Code)
	}

	// Test B: Authenticated admin access to restricted document (Minutes) -> 200 OK
	reqAdmin := httptest.NewRequest(http.MethodGet, "/documents/download?id="+strconv.Itoa(fileRecordID), nil)
	reqAdmin.AddCookie(&http.Cookie{Name: "admin_session", Value: "1"})
	wAdmin := httptest.NewRecorder()
	DownloadDocumentFile(wAdmin, reqAdmin)
	if wAdmin.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for admin download, got %d (body: %s)", wAdmin.Code, wAdmin.Body.String())
	}
	disposition := wAdmin.Header().Get("Content-Disposition")
	if !strings.Contains(disposition, "convention_minutes_2026.pdf") {
		t.Errorf("Expected Content-Disposition to contain filename, got %q", disposition)
	}
	if wAdmin.Body.String() != string(testContent) {
		t.Errorf("Expected body to match test content, got %q", wAdmin.Body.String())
	}

	// Test C: Download via document_id fallback
	reqDocID := httptest.NewRequest(http.MethodGet, "/documents/download?document_id="+strconv.Itoa(docID), nil)
	reqDocID.AddCookie(&http.Cookie{Name: "admin_session", Value: "1"})
	wDocID := httptest.NewRecorder()
	DownloadDocumentFile(wDocID, reqDocID)
	if wDocID.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for download by document_id, got %d", wDocID.Code)
	}

	// Test D: Missing file record -> 404
	reqMissing := httptest.NewRequest(http.MethodGet, "/documents/download?id=99999", nil)
	reqMissing.AddCookie(&http.Cookie{Name: "admin_session", Value: "1"})
	wMissing := httptest.NewRecorder()
	DownloadDocumentFile(wMissing, reqMissing)
	if wMissing.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for missing document file ID, got %d", wMissing.Code)
	}

	// Test E: Public document download by anonymous user -> 200 OK
	pubContent := []byte("Public convention programme guide")
	pubFilePath := filepath.Join(uploadDir, "public_guide.txt")
	_ = os.WriteFile(pubFilePath, pubContent, 0644)

	pubDoc := models.Document{
		Title:       "2026 Public Convention Programme",
		Category:    "Convention Programme",
		Convention:  "2026 Convention",
		Year:        2026,
		Description: "Schedule and guide",
		CreatedBy:   "Committee",
	}
	_ = repository.CreateDocument(pubDoc)
	allDocs, _ := repository.GetAllDocuments()
	var pubDocID int
	for _, d := range allDocs {
		if d.Title == "2026 Public Convention Programme" {
			pubDocID = d.ID
			break
		}
	}
	if pubDocID == 0 {
		t.Fatalf("Could not find created public document")
	}

	_ = repository.SaveDocumentFile(models.DocumentFile{
		DocumentID: pubDocID,
		FileName:   "public_guide.txt",
		FilePath:   pubFilePath,
		FileType:   "text/plain",
	})
	pubFiles, _ := repository.GetDocumentFiles(pubDocID)
	pubFileID := pubFiles[0].ID

	reqPub := httptest.NewRequest(http.MethodGet, "/documents/download?id="+strconv.Itoa(pubFileID), nil)
	wPub := httptest.NewRecorder()
	DownloadDocumentFile(wPub, reqPub)
	if wPub.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for public document download by anonymous user, got %d", wPub.Code)
	}
}
