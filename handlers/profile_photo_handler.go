package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"convention-management-system/services"
	"convention-management-system/sessions"
	"convention-management-system/utils"
)

func UploadProfilePhoto(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	session, _ := sessions.Store.Get(r, "youth-community")

	userID, ok := session.Values["user_id"].(int)
	if !ok || userID <= 0 {
		if isAPIRequest(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "Unauthorized"})
			return
		}
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	isJSON := strings.Contains(r.Header.Get("Content-Type"), "application/json")
	var base64Data string
	var consentGiven bool

	if isJSON {
		var reqBody struct {
			PhotoBase64 string `json:"photo_base64"`
			Consent     bool   `json:"consent"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err == nil {
			base64Data = reqBody.PhotoBase64
			consentGiven = reqBody.Consent
		}
	} else {
		base64Data = r.FormValue("photo_base64")
		consentGiven = r.FormValue("consent") == "true" || r.FormValue("consent") == "on" || r.FormValue("consent") == "1"
	}

	var savedFilename string

	if base64Data != "" {
		// 1. Process Base64 Photo from Assisted Facial Liveness Verification
		commaIdx := strings.Index(base64Data, ",")
		rawB64 := base64Data
		if commaIdx != -1 {
			rawB64 = base64Data[commaIdx+1:]
		}

		decodedBytes, err := base64.StdEncoding.DecodeString(rawB64)
		if err != nil {
			returnPhotoError(w, r, "Invalid image encoding", http.StatusBadRequest)
			return
		}

		if len(decodedBytes) < 512 {
			returnPhotoError(w, r, "Captured image file is too small or corrupt", http.StatusBadRequest)
			return
		}

		cfg, _, err := image.DecodeConfig(bytes.NewReader(decodedBytes))
		if err != nil {
			returnPhotoError(w, r, "Captured image is not a valid JPEG/PNG", http.StatusBadRequest)
			return
		}
		if cfg.Width < 80 || cfg.Height < 80 {
			returnPhotoError(w, r, "Captured image resolution is too low. Please center your face.", http.StatusBadRequest)
			return
		}

		destDir := filepath.Join(utils.GetUploadBaseDir(), "profiles")
		if err := os.MkdirAll(destDir, 0755); err != nil {
			returnPhotoError(w, r, "Failed creating directory", http.StatusInternalServerError)
			return
		}

		savedFilename = fmt.Sprintf("profile_%d_%d.jpg", userID, time.Now().UnixNano())
		fullPath := filepath.Join(destDir, savedFilename)
		if err := os.WriteFile(fullPath, decodedBytes, 0644); err != nil {
			returnPhotoError(w, r, "Failed saving captured photo", http.StatusInternalServerError)
			return
		}
		// Also write to local uploads/profiles if distinct, ensuring local fallback availability
		if utils.GetUploadBaseDir() != "uploads" {
			_ = os.MkdirAll("uploads/profiles", 0755)
			_ = os.WriteFile(filepath.Join("uploads/profiles", savedFilename), decodedBytes, 0644)
		}

		// Mirror to S3 when enabled
		if utils.IsObjectStorageEnabled() {
			_, _ = utils.UploadToObjectStorage("profiles/"+savedFilename, bytes.NewReader(decodedBytes), int64(len(decodedBytes)), "image/jpeg")
		}

		// Persist to durable database media storage
		if utils.MediaStorageSaver != nil {
			_ = utils.MediaStorageSaver("profiles/"+savedFilename, "image/jpeg", decodedBytes)
		}
	} else {
		// 2. Standard Multipart File Upload (Accessible Fallback)
		file, header, err := r.FormFile("profile_photo")
		if err != nil {
			returnPhotoError(w, r, "No photo selected or invalid upload", http.StatusBadRequest)
			return
		}
		defer file.Close()

		if header.Size <= 0 || header.Size > 10*1024*1024 {
			returnPhotoError(w, r, "File size must be between 1 byte and 10MB", http.StatusBadRequest)
			return
		}

		// Read head to check image format
		headBuf := make([]byte, 512)
		n, _ := file.Read(headBuf)
		_, _ = file.Seek(0, io.SeekStart)

		contentType := http.DetectContentType(headBuf[:n])
		if !strings.HasPrefix(contentType, "image/") {
			returnPhotoError(w, r, "Uploaded file must be a valid image", http.StatusBadRequest)
			return
		}

		savedFilename, err = utils.SaveUploadedFile(file, header, "uploads/profiles")
		if err != nil {
			returnPhotoError(w, r, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Clean up old profile photo
	oldProfilePhoto, _, err := services.GetProfilePhotos(userID)
	if err == nil && oldProfilePhoto != "" && oldProfilePhoto != savedFilename {
		utils.DeleteFile(filepath.Join("uploads/profiles", oldProfilePhoto))
	}

	// Update database
	if err := services.UpdateProfilePhoto(userID, savedFilename); err != nil {
		returnPhotoError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}

	if isAPIRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":       true,
			"profile_photo": savedFilename,
			"message":       "Profile photo updated and verified successfully.",
			"consent":       consentGiven,
		})
		return
	}

	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

func isAPIRequest(r *http.Request) bool {
	return r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
		strings.Contains(r.Header.Get("Accept"), "application/json") ||
		strings.Contains(r.Header.Get("Content-Type"), "application/json")
}

func returnPhotoError(w http.ResponseWriter, r *http.Request, message string, status int) {
	if isAPIRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   message,
		})
		return
	}
	http.Error(w, message, status)
}

func UploadCoverPhoto(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	session, _ := sessions.Store.Get(r, "youth-community")

	userID, ok := session.Values["user_id"].(int)
	if !ok || userID <= 0 {
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	file, header, err := r.FormFile("cover_photo")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	_, oldCoverPhoto, err := services.GetProfilePhotos(userID)
	if err == nil && oldCoverPhoto != "" {
		utils.DeleteFile(filepath.Join("uploads/covers", oldCoverPhoto))
	}

	filename, err := utils.SaveUploadedFile(file, header, "uploads/covers")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = services.UpdateCoverPhoto(userID, filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}
