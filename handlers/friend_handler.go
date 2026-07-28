package handlers

import (
	"net/http"
	"strconv"

	"convention-management-system/services"
	"convention-management-system/sessions"
)

func SendFriendRequest(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/community", http.StatusSeeOther)
		return
	}

	session, _ := sessions.Store.Get(r, "youth-community")

	senderID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	receiver := r.FormValue("receiver_id")
	if receiver == "" {
		receiver = r.FormValue("user_id")
	}

	receiverID, err := strconv.Atoi(receiver)
	if err != nil {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, "Invalid user", http.StatusBadRequest)
		return
	}

	err = services.SendFriendRequest(
		senderID,
		receiverID,
	)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if username := r.FormValue("username"); username != "" {
		http.Redirect(
			w,
			r,
			"/user/"+username,
			http.StatusSeeOther,
		)
		return
	}

	http.Redirect(
		w,
		r,
		"/discover",
		http.StatusSeeOther,
	)
}

func AcceptFriendRequest(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/notifications", http.StatusSeeOther)
		return
	}

	requestID, err := strconv.Atoi(
		r.FormValue("request_id"),
	)

	if err != nil {
		http.Error(
			w,
			"Invalid request",
			http.StatusBadRequest,
		)
		return
	}

	senderID, err := strconv.Atoi(
		r.FormValue("sender_id"),
	)

	if err != nil {
		http.Error(
			w,
			"Invalid sender",
			http.StatusBadRequest,
		)
		return
	}

	receiverID, err := strconv.Atoi(
		r.FormValue("receiver_id"),
	)

	if err != nil {
		http.Error(
			w,
			"Invalid receiver",
			http.StatusBadRequest,
		)
		return
	}

	err = services.AcceptFriendRequest(
		requestID,
		senderID,
		receiverID,
	)

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	http.Redirect(
		w,
		r,
		"/friend-requests",
		http.StatusSeeOther,
	)
}

func DeclineFriendRequest(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/friend-requests", http.StatusSeeOther)
		return
	}

	requestID, err := strconv.Atoi(
		r.FormValue("request_id"),
	)

	if err != nil {
		http.Error(
			w,
			"Invalid request",
			http.StatusBadRequest,
		)
		return
	}

	err = services.DeclineFriendRequest(
		requestID,
	)

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	http.Redirect(
		w,
		r,
		"/friend-requests",
		http.StatusSeeOther,
	)
}

func CancelFriendRequest(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/community", http.StatusSeeOther)
		return
	}

	session, _ := sessions.Store.Get(r, "youth-community")

	senderID, ok := session.Values["user_id"].(int)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Accept either receiver_id (Discover page)
	// or user_id (Public Profile page)
	receiver := r.FormValue("receiver_id")
	if receiver == "" {
		receiver = r.FormValue("user_id")
	}

	receiverID, err := strconv.Atoi(receiver)
	if err != nil {
		http.Error(
			w,
			"Invalid user",
			http.StatusBadRequest,
		)
		return
	}

	err = services.CancelFriendRequest(
		senderID,
		receiverID,
	)

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	// Return to the page the request came from
	if username := r.FormValue("username"); username != "" {
		http.Redirect(
			w,
			r,
			"/user/"+username,
			http.StatusSeeOther,
		)
		return
	}

	http.Redirect(
		w,
		r,
		"/discover",
		http.StatusSeeOther,
	)
}

func RemoveFriend(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/community", http.StatusSeeOther)
		return
	}

	session, _ := sessions.Store.Get(r, "youth-community")

	user1ID, ok := session.Values["user_id"].(int)

	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user2ID, err := strconv.Atoi(
		r.FormValue("user_id"),
	)

	if err != nil {
		http.Error(
			w,
			"Invalid user",
			http.StatusBadRequest,
		)
		return
	}

	err = services.RemoveFriend(
		user1ID,
		user2ID,
	)

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}

	http.Redirect(
		w,
		r,
		"/user/"+r.FormValue("username"),
		http.StatusSeeOther,
	)
}
