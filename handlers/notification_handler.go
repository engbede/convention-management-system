package handlers

import (
	"net/http"

	"convention-management-system/services"
	"convention-management-system/sessions"
)

func Notifications(w http.ResponseWriter, r *http.Request) {

	session, _ := sessions.Store.Get(
		r,
		"youth-community",
	)

	userID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	notifications, err := services.GetNotificationsByUser(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	unread, _ := services.CountUnreadNotifications(userID)

	Render(
		w,
		"notifications.html",
		map[string]any{
			"Notifications": notifications,
			"UnreadCount":   unread,
		},
	)
}
