package handlers

import (
        "net/http"
        "strconv"

        "convention-management-system/models"
        "convention-management-system/services"
        "convention-management-system/sessions"
)

func FollowUser(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/community", http.StatusSeeOther)
		return
	}

	session, _ := sessions.Store.Get(r, "youth-community")

	followerID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	followingID, err := strconv.Atoi(r.FormValue("user_id"))
	if err != nil {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	if followerID == followingID {
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	err = services.FollowUser(followerID, followingID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	notification := models.Notification{
        SenderID:   followerID,
        ReceiverID: followingID,
        Type:       "follow",
        Message:    "started following you.",
}

_ = services.CreateNotification(notification)

	err = services.NotifyFollow(
		followerID,
		followingID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(
		w,
		r,
		"/user/"+r.FormValue("username"),
		http.StatusSeeOther,
	)
}

func UnfollowUser(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/community", http.StatusSeeOther)
		return
	}

	session, _ := sessions.Store.Get(r, "youth-community")

	followerID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	followingID, err := strconv.Atoi(r.FormValue("user_id"))
	if err != nil {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	err = services.UnfollowUser(followerID, followingID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(
		w,
		r,
		"/user/"+r.FormValue("username"),
		http.StatusSeeOther,
	)
}
