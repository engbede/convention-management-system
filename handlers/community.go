package handlers

import (
	"net/http"

	"convention-management-system/repository"
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
	pendingRequestsCount := 0
	var viewerUserID int

	session, _ := sessions.Store.Get(
		r,
		"youth-community",
	)

	var currentUser any
	if userID, ok := session.Values["user_id"].(int); ok {
		viewerUserID = userID
		unread, _ = services.CountUnreadNotifications(userID)
		if pending, pErr := services.GetPendingFriendRequests(userID); pErr == nil {
			pendingRequestsCount = len(pending)
		}
		if u, uErr := repository.GetUserByID(userID); uErr == nil {
			currentUser = u
		}
	}

	sidebarData, _ := services.GetCommunitySidebarData(viewerUserID, false)

	data := map[string]any{
		"Posts":                      posts,
		"UnreadCount":                unread,
		"Sidebar":                    sidebarData,
		"CurrentUser":                currentUser,
		"CurrentUserID":              viewerUserID,
		"PendingFriendRequestsCount": pendingRequestsCount,
		"ActiveNav":                  "feed",
	}

	Render(
		w,
		"community.html",
		data,
	)
}
