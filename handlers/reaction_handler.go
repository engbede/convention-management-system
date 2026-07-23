package handlers

import (
	"net/http"
	"strconv"

	"convention-management-system/services"
	"convention-management-system/sessions"
)

func ReactToPost(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {

		http.Redirect(
			w,
			r,
			"/community",
			http.StatusSeeOther,
		)

		return
	}

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

	postID, err := strconv.Atoi(
		r.FormValue("post_id"),
	)

	if err != nil {

		http.Error(
			w,
			"Invalid post ID",
			http.StatusBadRequest,
		)

		return
	}

	reaction := r.FormValue("reaction")

	err = services.ReactToPost(
		postID,
		userID,
		reaction,
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
		"/community",
		http.StatusSeeOther,
	)
}
