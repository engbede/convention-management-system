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

	println("================================")
	println("Notification page opened")
	println("Logged-in user ID =", userID)
	println("================================")

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

	println("================================")
	println("Handler received:", len(notifications), "notifications")
	println("Unread:", unread)
	println("================================")

	Render(
		w,
		"notifications.html",
		map[string]any{
			"Notifications": notifications,
			"UnreadCount":   unread,
		},
	)
}
