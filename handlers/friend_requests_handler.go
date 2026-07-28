package handlers

import (
	"net/http"

	"convention-management-system/services"
	"convention-management-system/sessions"
)

func FriendRequests(w http.ResponseWriter, r *http.Request) {

	session, _ := sessions.Store.Get(r, "youth-community")

	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := services.GetUserProfile(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	requests, err := services.GetPendingFriendRequests(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	unread, _ := services.CountUnreadNotifications(userID)

	Render(
		w,
		"friend_requests.html",
		map[string]any{
			"User":        user,
			"Requests":    requests,
			"UnreadCount": unread,
		},
	)
}
