package handlers

import (
	"net/http"

	"convention-management-system/sessions"
)

func AdminLogout(
	w http.ResponseWriter,
	r *http.Request,
) {
	http.SetCookie(
		w,
		&http.Cookie{
			Name:   "admin_session",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		},
	)

	if session, err := sessions.Store.Get(r, "youth-community"); err == nil {
		session.Options.MaxAge = -1
		_ = session.Save(r, w)
	}

	http.Redirect(
		w,
		r,
		"/login",
		http.StatusSeeOther,
	)
}
