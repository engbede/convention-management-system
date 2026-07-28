package handlers

import (
	"net/http"

	"convention-management-system/services"
	"convention-management-system/sessions"
)

func FriendsDashboard(
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


	user, err := services.GetUserProfile(
		userID,
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}


	friends, _ := services.GetFriends(
		userID,
	)


	incoming, _ := services.GetPendingFriendRequests(
		userID,
	)


	outgoing, _ := services.GetSentFriendRequests(
		userID,
	)


	unread, _ := services.CountUnreadNotifications(
		userID,
	)


	Render(
		w,
		"friends_dashboard.html",
		map[string]any{

			"User": user,

			"Friends": friends,

			"Incoming": incoming,

			"Outgoing": outgoing,

			"UnreadCount": unread,

		},
	)
}