package handlers

import (
	"net/http"
	"strings"

	"convention-management-system/database"
	"convention-management-system/models"
	"convention-management-system/services"
	"convention-management-system/sessions"
)

func PublicProfile(
	w http.ResponseWriter,
	r *http.Request,
) {

	// URL format: /user/username
	username := strings.TrimPrefix(
		r.URL.Path,
		"/user/",
	)

	if username == "" {

		http.NotFound(w, r)

		return
	}

	// Logged-in user
	session, _ := sessions.Store.Get(
		r,
		"youth-community",
	)

	currentUserID, _ := session.Values["user_id"].(int)

	// User being viewed
	user, err := services.GetUserByUsername(username)
	if err != nil {

		http.NotFound(w, r)

		return
	}

	posts, err := services.GetPostsByUser(user.ID)
	if err != nil {
		posts = []models.Post{}
	}

	isFollowing := false

	if currentUserID != 0 && currentUserID != user.ID {

		isFollowing, _ = services.IsFollowing(
			currentUserID,
			user.ID,
		)

	}

	friendStatus := "none"
	var incomingRequestID int
	if currentUserID != 0 && currentUserID != user.ID {
		friendStatus, _ = services.GetFriendStatus(currentUserID, user.ID)
		if friendStatus == "received" && database.DB != nil {
			_ = database.DB.QueryRow(`
				SELECT id FROM friend_requests
				WHERE sender_id = $1 AND receiver_id = $2 AND status = 'pending'
			`, user.ID, currentUserID).Scan(&incomingRequestID)
		}
	}

	unread := 0

	if currentUserID != 0 {
		unread, _ = services.CountUnreadNotifications(currentUserID)
	}

	Render(
		w,
		"public_profile.html",
		map[string]any{
			"User":              user,
			"Posts":             posts,
			"CurrentUserID":     currentUserID,
			"IsFollowing":       isFollowing,
			"FriendStatus":      friendStatus,
			"IncomingRequestID": incomingRequestID,
			"UnreadCount":       unread,
		},
	)
}
