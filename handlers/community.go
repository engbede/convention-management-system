package handlers

import (
	"net/http"

	"convention-management-system/services"
	"convention-management-system/sessions"
)

func Community(
	w http.ResponseWriter,
	r *http.Request,
) {

	posts, err := services.GetCommunityFeed()
	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	unread := 0
	var viewerUserID int

	session, _ := sessions.Store.Get(
		r,
		"youth-community",
	)

	if userID, ok := session.Values["user_id"].(int); ok {
		viewerUserID = userID
		unread, _ = services.CountUnreadNotifications(userID)
	}

	sidebarData, _ := services.GetCommunitySidebarData(viewerUserID, false)

	data := map[string]any{
		"Posts":         posts,
		"UnreadCount":   unread,
		"Sidebar":       sidebarData,
		"CurrentUserID": viewerUserID,
	}

	Render(
		w,
		"community.html",
		data,
	)
}
