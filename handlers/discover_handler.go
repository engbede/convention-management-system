package handlers

import (
	"net/http"

	"convention-management-system/services"
	"convention-management-system/sessions"
)

func Discover(
	w http.ResponseWriter,
	r *http.Request,
) {

	session, _ := sessions.Store.Get(
		r,
		"youth-community",
	)

	currentUserID, ok := session.Values["user_id"].(int)
	if !ok {

		http.Redirect(
			w,
			r,
			"/login",
			http.StatusSeeOther,
		)

		return
	}

	user, err := services.GetUserProfile(currentUserID)
	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	users, err := services.GetDiscoverUsers(currentUserID)
	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	// Determine friendship status for each discovered user.
	for i := range users {

		status, err := services.GetFriendStatus(
			currentUserID,
			users[i].ID,
		)

		if err == nil {
			users[i].FriendStatus = status
		}
	}

	unread, _ := services.CountUnreadNotifications(currentUserID)

	Render(
		w,
		"discover.html",
		map[string]any{
			"User":        user,
			"Users":       users,
			"UnreadCount": unread,
		},
	)
}
