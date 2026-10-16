package handlers

import (
	"net/http"
	"strconv"

	"convention-management-system/models"
	"convention-management-system/repository"
	"convention-management-system/sessions"
)

type HomeViewData struct {
	Convention          *models.Convention
	AllConventions      []models.Convention
	Notices             []models.Notice
	CurrentUser         *models.User
	IsLoggedIn          bool
	IsAdmin             bool
	UnreadMessagesCount int
	UnreadNotifications int
}

func Home(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	var activeConvention *models.Convention
	convention, err := repository.GetActiveConvention()
	if err == nil && convention.ID > 0 {
		activeConvention = &convention
	}

	allConventions, _ := repository.GetAllConventions()
	notices, _ := repository.GetActiveNotices()

	// Check community member session
	var currentUser *models.User
	isLoggedIn := false
	unreadMessages := 0
	unreadNotifications := 0

	session, err := sessions.Store.Get(r, "youth-community")
	if err == nil {
		if userID, ok := session.Values["user_id"].(int); ok && userID > 0 {
			user, err := repository.GetUserByID(userID)
			if err == nil && user.ID > 0 {
				currentUser = &user
				isLoggedIn = true
				unreadMessages, _ = repository.CountTotalUnreadMessages(userID)
				unreadNotifications, _ = repository.CountUnreadNotifications(userID)
			}
		}
	}

	// Check admin session
	isAdmin := false
	if cookie, err := r.Cookie("admin_session"); err == nil {
		if id, err := strconv.Atoi(cookie.Value); err == nil && id > 0 {
			isAdmin = true
		}
	}

	data := HomeViewData{
		Convention:          activeConvention,
		AllConventions:      allConventions,
		Notices:             notices,
		CurrentUser:         currentUser,
		IsLoggedIn:          isLoggedIn,
		IsAdmin:             isAdmin,
		UnreadMessagesCount: unreadMessages,
		UnreadNotifications: unreadNotifications,
	}

	Render(
		w,
		"home.html",
		data,
	)
}
