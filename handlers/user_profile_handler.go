package handlers

import (
	"net/http"
	"strings"

	"convention-management-system/models"
	"convention-management-system/services"
	"convention-management-system/sessions"
)

func UserProfile(
	w http.ResponseWriter,
	r *http.Request,
) {

	// Expected URL:
	// /user/emmanuel

	username := strings.TrimPrefix(
		r.URL.Path,
		"/user/",
	)

	if username == "" {
		http.NotFound(w, r)
		return
	}

	user, err := services.GetUserByUsername(username)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	posts, err := services.GetPostsByUser(user.ID)
	if err != nil {
		posts = []models.Post{}
	}

	session, _ := sessions.Store.Get(
		r,
		"youth-community",
	)

	currentUserID, _ := session.Values["user_id"].(int)

	isOwnProfile := currentUserID == user.ID

	isFollowing := false

	if !isOwnProfile {

		isFollowing, _ = services.IsFollowing(
			currentUserID,
			user.ID,
		)

	}
	Render(
		w,
		"profile.html",
		map[string]any{
			"User":         user,
			"Posts":        posts,
			"IsOwnProfile": isOwnProfile,
			"IsFollowing":  isFollowing,
		},
	)
}
