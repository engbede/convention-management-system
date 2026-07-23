package handlers

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"convention-management-system/models"
	"convention-management-system/services"
	"convention-management-system/sessions"
)

func CreateComment(
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

	session, _ := sessions.Store.Get(
		r,
		"youth-community",
	)

	userID, ok := session.Values["user_id"].(int)
	if !ok {

		http.Redirect(
			w,
			r,
			"/login",
			http.StatusSeeOther,
		)
		return
	}

	postID, err := strconv.Atoi(
		r.FormValue("post_id"),
	)
	log.Println("Post ID:", postID)

	if err != nil {

		http.Error(
			w,
			"Invalid post ID",
			http.StatusBadRequest,
		)
		return
	}

	content := strings.TrimSpace(
		r.FormValue("content"),
	)

	log.Println("Comment:", content)

	if content == "" {

		http.Redirect(
			w,
			r,
			"/community",
			http.StatusSeeOther,
		)
		return
	}

	comment := &models.Comment{
		PostID:  postID,
		UserID:  userID,
		Content: content,
	}

	err = services.CreateComment(comment)

	if err == nil {
		log.Println("Comment created successfully")
	}

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	http.Redirect(
		w,
		r,
		"/community",
		http.StatusSeeOther,
	)
}
