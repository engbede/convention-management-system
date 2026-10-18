package handlers

import (
	"log"
	"net/http"
	"strings"

	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/sessions"
	"convention-management-system/utils"
)

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

	log.Println("Session values:", session.Values)

	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Error(w, "User session is invalid", http.StatusUnauthorized)
		return
	}

	_ = r.ParseMultipartForm(10 << 20) // 10 MB

	content := strings.TrimSpace(r.FormValue("content"))
	visibility := strings.TrimSpace(r.FormValue("visibility"))
	if visibility == "" {
		visibility = "public"
	}

	var imageName string
	file, header, err := r.FormFile("image")
	if err == nil && file != nil && header != nil {
		savedName, saveErr := utils.SaveUploadedFile(file, header, "uploads/posts")
		if saveErr != nil {
			log.Println("Image upload warning:", saveErr)
		} else {
			imageName = "posts/" + savedName
		}
	}

	if content == "" && imageName == "" {
		http.Redirect(w, r, "/community", http.StatusSeeOther)
		return
	}

	post := models.Post{
		UserID:     userID,
		Content:    content,
		Image:      imageName,
		Visibility: visibility,
	}

	err = repository.CreatePost(&post)
	if err != nil {
		log.Println("CreatePost error:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println("Post created successfully")

	http.Redirect(w, r, "/community", http.StatusSeeOther)
}
