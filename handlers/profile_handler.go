package handlers

import (
	"net/http"

	"convention-management-system/models"
	"convention-management-system/services"
	"convention-management-system/sessions"
)

func Profile(
	w http.ResponseWriter,
	r *http.Request,
) {

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

	user, err := services.GetUserProfile(userID)
	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	// This is my own profile
	user.IsOwner = true

	// Social statistics
	user.FriendsCount, _ = services.GetFriendCount(user.ID)

	posts, err := services.GetPostsByUser(user.ID)
	if err != nil {
		posts = []models.Post{}
	}

	user.PostsCount = len(posts)

	unread, _ := services.CountUnreadNotifications(userID)

	Render(
		w,
		"profile.html",
		map[string]any{
			"User":        user,
			"Posts":       posts,
			"UnreadCount": unread,
		},
	)
}
