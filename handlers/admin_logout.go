package handlers

import "net/http"

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

	http.Redirect(
		w,
		r,
		"/login",
		http.StatusSeeOther,
	)
}
