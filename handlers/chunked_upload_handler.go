package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"convention-management-system/services"
	"convention-management-system/sessions"
	"convention-management-system/utils"
)

func getUserIDFromSession(r *http.Request) (int, bool) {
	session, err := sessions.Store.Get(r, "youth-community")
	if err != nil {
		return 0, false
	}
	userID, ok := session.Values["user_id"].(int)
	return userID, ok && userID > 0
}

func sendJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func sendJSONError(w http.ResponseWriter, status int, message string) {
	sendJSON(w, status, map[string]any{
		"success": false,
		"error":   message,
	})
}

// GetUploadConfigAPI returns upload limits and chunk settings for the frontend.
func GetUploadConfigAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	sendJSON(w, http.StatusOK, map[string]any{
		"success":              true,
		"max_video_size_bytes": utils.GetMaxVideoUploadSize(),
		"max_video_size_mb":    utils.GetMaxVideoUploadSizeMB(),
		"chunk_size_bytes":     services.DefaultChunkSize,
		"s3_enabled":           utils.IsObjectStorageEnabled(),
		"allowed_extensions":   []string{".mp4", ".webm", ".mov", ".mkv", ".avi"},
	})
}

// InitiateUploadAPI validates and starts an upload session.
func InitiateUploadAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID, ok := getUserIDFromSession(r)
	if !ok {
		sendJSONError(w, http.StatusUnauthorized, "User session required to upload media")
		return
	}

	var req struct {
		Filename    string `json:"filename"`
		FileSize    int64  `json:"file_size"`
		ContentType string `json:"content_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	session, err := services.InitiateChunkedUpload(userID, req.Filename, req.FileSize, req.ContentType)
	if err != nil {
		sendJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	sendJSON(w, http.StatusOK, map[string]any{
		"success":         true,
		"upload_id":       session.UploadID,
		"key":             session.TargetKey,
		"chunk_size":      session.ChunkSize,
		"total_chunks":    session.TotalChunks,
		"uploaded_chunks": []int{},
	})
}

// UploadChunkAPI receives a single chunk of video data.
func UploadChunkAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID, ok := getUserIDFromSession(r)
	if !ok {
		sendJSONError(w, http.StatusUnauthorized, "User session required")
		return
	}

	// Limit single chunk upload body to 20 MB (10 MB chunk + headers)
	r.Body = http.MaxBytesReader(w, r.Body, 20<<20)

	if err := r.ParseMultipartForm(12 << 20); err != nil {
		sendJSONError(w, http.StatusBadRequest, "Failed to parse chunk multipart data")
		return
	}

	uploadID := strings.TrimSpace(r.FormValue("upload_id"))
	if uploadID == "" {
		sendJSONError(w, http.StatusBadRequest, "Missing upload_id")
		return
	}

	chunkIdxStr := strings.TrimSpace(r.FormValue("chunk_index"))
	chunkIndex, err := strconv.Atoi(chunkIdxStr)
	if err != nil || chunkIndex < 0 {
		sendJSONError(w, http.StatusBadRequest, "Invalid chunk_index")
		return
	}

	file, header, err := r.FormFile("chunk")
	if err != nil || file == nil {
		sendJSONError(w, http.StatusBadRequest, "Missing chunk file payload")
		return
	}
	defer file.Close()

	if err := services.SaveChunk(userID, uploadID, chunkIndex, file, header.Size); err != nil {
		sendJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sendJSON(w, http.StatusOK, map[string]any{
		"success":        true,
		"upload_id":      uploadID,
		"chunk_index":    chunkIndex,
		"bytes_received": header.Size,
	})
}

// CompleteUploadAPI finalizes the chunked upload, verifies integrity, and saves the final file.
func CompleteUploadAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID, ok := getUserIDFromSession(r)
	if !ok {
		sendJSONError(w, http.StatusUnauthorized, "User session required")
		return
	}

	var req struct {
		UploadID string `json:"upload_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UploadID == "" {
		sendJSONError(w, http.StatusBadRequest, "Missing upload_id")
		return
	}

	targetKey, finalURL, err := services.CompleteChunkedUpload(userID, req.UploadID)
	if err != nil {
		sendJSONError(w, http.StatusBadRequest, fmt.Sprintf("Failed to finalize upload: %v", err))
		return
	}

	sendJSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"video_key": targetKey,
		"video_url": finalURL,
		"message":   "Video upload completed and verified",
	})
}

// AbortUploadAPI cancels an ongoing upload session and removes temporary chunks.
func AbortUploadAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID, ok := getUserIDFromSession(r)
	if !ok {
		sendJSONError(w, http.StatusUnauthorized, "User session required")
		return
	}

	var req struct {
		UploadID string `json:"upload_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UploadID == "" {
		sendJSONError(w, http.StatusBadRequest, "Missing upload_id")
		return
	}

	if err := services.AbortChunkedUpload(userID, req.UploadID); err != nil {
		sendJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sendJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Upload aborted and cleaned up",
	})
}

// GetUploadStatusAPI returns which chunks have been received for an upload ID, enabling resumes.
func GetUploadStatusAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID, ok := getUserIDFromSession(r)
	if !ok {
		sendJSONError(w, http.StatusUnauthorized, "User session required")
		return
	}

	uploadID := strings.TrimSpace(r.URL.Query().Get("upload_id"))
	if uploadID == "" {
		sendJSONError(w, http.StatusBadRequest, "Missing upload_id")
		return
	}

	chunks, err := services.GetUploadedChunks(userID, uploadID)
	if err != nil {
		sendJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	sendJSON(w, http.StatusOK, map[string]any{
		"success":         true,
		"upload_id":       uploadID,
		"uploaded_chunks": chunks,
	})
}

