package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/sessions"
	"convention-management-system/utils"
)

func isCommunityPostAPIRequest(r *http.Request) bool {
	return r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
		strings.Contains(r.Header.Get("Accept"), "application/json") ||
		strings.Contains(r.Header.Get("Content-Type"), "application/json")
}

func returnCommunityPostError(w http.ResponseWriter, r *http.Request, message string, status int) {
	if isCommunityPostAPIRequest(r) {
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

func CreateCommunityPost(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		http.Redirect(
			w,
			r,
			"/community",
			http.StatusSeeOther,
		)
		return
	}

	session, _ := sessions.Store.Get(r, "youth-community")
	userID, ok := session.Values["user_id"].(int)
	if !ok || userID <= 0 {
		returnCommunityPostError(w, r, "User session is invalid", http.StatusUnauthorized)
		return
	}

	// Bound max request body based on configured maximum video size (default 500 MB) + 25 MB overhead
	maxVideoBytes := utils.GetMaxVideoUploadSize()
	maxBodyBytes := maxVideoBytes + (25 << 20)
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	// Keep memory buffer lean at 16 MB so server RAM is conserved while streaming to disk
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		log.Println("ParseMultipartForm error:", err)
		errMsg := fmt.Sprintf("Upload exceeds the maximum allowed size of %d MB or request structure is invalid", utils.GetMaxVideoUploadSizeMB())
		returnCommunityPostError(w, r, errMsg, http.StatusBadRequest)
		return
	}

	content := strings.TrimSpace(r.FormValue("content"))
	visibility := strings.TrimSpace(r.FormValue("visibility"))
	switch visibility {
	case "public", "members", "private":
	default:
		visibility = "public"
	}

	var imageName string
	imgFile, imgHeader, imgErr := r.FormFile("image")
	if imgErr == nil && imgFile != nil && imgHeader != nil {
		if imgHeader.Size > 0 {
			savedName, saveErr := utils.SaveUploadedFile(imgFile, imgHeader, "uploads/posts")
			if saveErr != nil {
				log.Println("Image upload error:", saveErr)
				returnCommunityPostError(w, r, saveErr.Error(), http.StatusBadRequest)
				return
			}
			imageName = "posts/" + savedName
		} else {
			_ = imgFile.Close()
		}
	} else if imgFile != nil {
		_ = imgFile.Close()
	}

	var videoName string
	vidFile, vidHeader, vidErr := r.FormFile("video")
	if vidErr == nil && vidFile != nil && vidHeader != nil {
		if vidHeader.Size > 0 {
			savedName, saveErr := utils.SaveUploadedVideo(vidFile, vidHeader, "uploads/posts")
			if saveErr != nil {
				log.Println("Video upload error:", saveErr)
				// Clean up orphaned image uploaded in this request if video validation fails
				if imageName != "" {
					_ = utils.DeleteFile("uploads/" + imageName)
				}
				returnCommunityPostError(w, r, saveErr.Error(), http.StatusBadRequest)
				return
			}
			videoName = "posts/" + savedName
		} else {
			_ = vidFile.Close()
		}
	} else if vidFile != nil {
		_ = vidFile.Close()
	}

	// If no direct file was uploaded, check if video was pre-uploaded via chunked/multipart upload session
	if videoName == "" {
		if videoKey := strings.TrimSpace(r.FormValue("video_key")); videoKey != "" {
			cleanKey := filepath.Clean("/" + videoKey)
			cleanKey = strings.TrimPrefix(cleanKey, "/")
			if strings.HasPrefix(cleanKey, "posts/") && !strings.Contains(cleanKey, "..") {
				diskPath := filepath.Join(utils.GetUploadBaseDir(), cleanKey)
				if _, err := os.Stat(diskPath); err == nil || utils.IsObjectStorageEnabled() {
					videoName = cleanKey
				} else {
					log.Println("Referenced video_key not found in storage:", cleanKey)
				}
			}
		}
	}

	if content == "" && imageName == "" && videoName == "" {
		returnCommunityPostError(w, r, "Post cannot be empty. Please enter text or attach an image or video.", http.StatusBadRequest)
		return
	}

	post := models.Post{
		UserID:     userID,
		Content:    content,
		Image:      imageName,
		Video:      videoName,
		Visibility: visibility,
	}

	err := repository.CreatePost(&post)
	if err != nil {
		log.Println("CreatePost error:", err)
		// Clean up newly created files so database failure does not leave orphan media
		if imageName != "" {
			_ = utils.DeleteFile("uploads/" + imageName)
		}
		if videoName != "" {
			_ = utils.DeleteFile("uploads/" + videoName)
		}
		returnCommunityPostError(w, r, "Failed to publish post. Please try again.", http.StatusInternalServerError)
		return
	}

	log.Println("Post created successfully with id:", post.ID)

	if isCommunityPostAPIRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":  true,
			"post_id":  post.ID,
			"redirect": "/community",
			"message":  "Post published successfully",
		})
		return
	}

	http.Redirect(w, r, "/community", http.StatusSeeOther)
}
