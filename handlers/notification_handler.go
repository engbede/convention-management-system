package handlers

import (
	"net/http"

	"convention-management-system/repository"
	"convention-management-system/services"
	"convention-management-system/sessions"
)

// Notifications renders the notification list page and automatically marks all
// unread notifications as read so that the badge count updates accurately.
func Notifications(w http.ResponseWriter, r *http.Request) {
	session, _ := sessions.Store.Get(
		r,
		"youth-community",
	)

	userID, ok := session.Values["user_id"].(int)
	if !ok || userID <= 0 {
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	notifications, err := services.GetNotificationsByUser(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	unread, _ := services.CountUnreadNotifications(userID)

	// Mark notifications as read once viewed
	_ = services.MarkNotificationsAsRead(userID)

	currentUser, _ := repository.GetUserByID(userID)

	Render(
		w,
		"notifications.html",
		map[string]any{
			"Notifications": notifications,
			"UnreadCount":   unread,
			"CurrentUser":   currentUser,
			"CurrentUserID": userID,
		},
	)
}

// MarkNotificationsReadAPI marks all unread notifications for the authenticated user as read.
// Supports both AJAX/JSON requests and standard form posts / redirects.
func MarkNotificationsReadAPI(w http.ResponseWriter, r *http.Request) {
	session, _ := sessions.Store.Get(r, "youth-community")
	userID, ok := session.Values["user_id"].(int)
	if !ok || userID <= 0 {
		if isFriendJSON(r) {
			sendFriendJSONError(w, http.StatusUnauthorized, "Authentication required")
			return
		}
		http.Redirect(w, r, "/community/login", http.StatusSeeOther)
		return
	}

	if err := services.MarkNotificationsAsRead(userID); err != nil {
		if isFriendJSON(r) {
			sendFriendJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if isFriendJSON(r) {
		sendFriendJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": "Notifications marked as read",
		})
		return
	}

	if ref := r.Header.Get("Referer"); ref != "" {
		http.Redirect(w, r, ref, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/notifications", http.StatusSeeOther)
}
