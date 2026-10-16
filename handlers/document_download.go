package handlers

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/sessions"
)

// isRestrictedCategory returns true if a document category requires authentication.
func isRestrictedCategory(category string) bool {
	c := strings.ToLower(strings.TrimSpace(category))
	switch c {
	case "minutes", "financial report", "attendance report", "committee report", "resolution":
		return true
	default:
		return false
	}
}

// resolveSafeFilePath locates the file on disk while preventing path traversal.
func resolveSafeFilePath(storedPath, fileName string) (string, error) {
	// Candidate search locations
	candidates := []string{}

	if storedPath != "" {
		candidates = append(candidates, storedPath)
		// Try clean relative path from current working directory
		cleanedStored := filepath.Clean(storedPath)
		candidates = append(candidates, cleanedStored)
		candidates = append(candidates, filepath.Join(".", cleanedStored))
	}

	if fileName != "" {
		baseName := filepath.Base(fileName)
		candidates = append(candidates,
			filepath.Join("uploads/documents", baseName),
			filepath.Join("uploads", baseName),
			filepath.Join(".", "uploads/documents", baseName),
		)
	}

	for _, cand := range candidates {
		cleaned := filepath.Clean(cand)
		// Ensure it doesn't escape allowed folders
		if strings.Contains(cleaned, "..") {
			continue
		}
		if info, err := os.Stat(cleaned); err == nil && !info.IsDir() {
			return cleaned, nil
		}
	}

	return "", os.ErrNotExist
}

// DownloadDocumentFile handles safe, authenticated downloading of documents and minutes.
func DownloadDocumentFile(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	fileIDStr := query.Get("file_id")
	if fileIDStr == "" {
		fileIDStr = query.Get("id")
	}
	docIDStr := query.Get("document_id")

	var targetFile *models.DocumentFile
	var targetDoc *models.Document

	// 1. Try resolving by File ID
	if fileIDStr != "" {
		if id, err := strconv.Atoi(fileIDStr); err == nil && id > 0 {
			file, err := repository.GetDocumentFileByID(id)
			if err == nil && file.ID > 0 {
				targetFile = &file
				if doc, err := repository.GetDocumentByID(file.DocumentID); err == nil && doc.ID > 0 {
					targetDoc = &doc
				}
			} else {
				// The ID might have been a Document ID instead of a DocumentFile ID
				if doc, err := repository.GetDocumentByID(id); err == nil && doc.ID > 0 {
					targetDoc = &doc
					files, err := repository.GetDocumentFiles(id)
					if err == nil && len(files) > 0 {
						targetFile = &files[0]
					}
				}
			}
		}
	}

	// 2. If still nil, try resolving by Document ID
	if targetFile == nil && docIDStr != "" {
		if docID, err := strconv.Atoi(docIDStr); err == nil && docID > 0 {
			if doc, err := repository.GetDocumentByID(docID); err == nil && doc.ID > 0 {
				targetDoc = &doc
				files, err := repository.GetDocumentFiles(docID)
				if err == nil && len(files) > 0 {
					targetFile = &files[0]
				}
			}
		}
	}

	if targetFile == nil {
		http.Error(w, "Document file not found.", http.StatusNotFound)
		return
	}

	// 3. Authorization check
	isAdmin := false
	if cookie, err := r.Cookie("admin_session"); err == nil {
		if id, err := strconv.Atoi(cookie.Value); err == nil && id > 0 {
			isAdmin = true
		}
	}

	isMember := false
	if session, err := sessions.Store.Get(r, "youth-community"); err == nil {
		if uid, ok := session.Values["user_id"].(int); ok && uid > 0 {
			isMember = true
		}
	}

	category := ""
	if targetDoc != nil {
		category = targetDoc.Category
	}

	if isRestrictedCategory(category) && !isAdmin && !isMember {
		http.Redirect(w, r, "/community/login?next="+r.URL.RequestURI(), http.StatusSeeOther)
		return
	}

	// 4. Safe file resolution on disk
	actualPath, err := resolveSafeFilePath(targetFile.FilePath, targetFile.FileName)
	if err != nil {
		http.Error(w, "The requested document file is not available on server storage.", http.StatusNotFound)
		return
	}

	f, err := os.Open(actualPath)
	if err != nil {
		http.Error(w, "Unable to open document file.", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "Unable to read document metadata.", http.StatusInternalServerError)
		return
	}

	// 5. Determine Content-Type and download headers
	contentType := targetFile.FileType
	if contentType == "" || contentType == "application/octet-stream" {
		ext := filepath.Ext(targetFile.FileName)
		contentType = mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
	}

	cleanFileName := strings.ReplaceAll(targetFile.FileName, `"`, `_`)
	cleanFileName = strings.ReplaceAll(cleanFileName, "\n", "")
	cleanFileName = strings.ReplaceAll(cleanFileName, "\r", "")

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, cleanFileName))
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))

	http.ServeContent(w, r, cleanFileName, stat.ModTime(), f)
}
