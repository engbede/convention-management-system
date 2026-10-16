package handlers

import (
	"net/http"

	"convention-management-system/sessions"
)

func CommunityLogout(
	w http.ResponseWriter,
	r *http.Request,
) {
	session, err := sessions.Store.Get(r, "youth-community")
	if err == nil {
		session.Options.MaxAge = -1
		_ = session.Save(r, w)
	}

	http.Redirect(
		w,
		r,
		"/community/login",
		http.StatusSeeOther,
	)
}
