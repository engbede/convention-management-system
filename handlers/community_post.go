package handlers

import (
	"log"
	"net/http"

	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/sessions"
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

	content := r.FormValue("content")

	log.Println("UserID:", userID)
	log.Println("Content:", content)

	post := models.Post{
		UserID:     userID,
		Content:    content,
		Visibility: "public",
	}

	err := repository.CreatePost(&post)
	if err != nil {
		log.Println("CreatePost error:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println("Post created successfully")

	http.Redirect(w, r, "/community", http.StatusSeeOther)
}
