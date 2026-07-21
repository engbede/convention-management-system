package handlers

import "net/http"

func CommunityLogout(
	w http.ResponseWriter,
	r *http.Request,
) {
	http.Redirect(
		w,
		r,
		"/community/login",
		http.StatusSeeOther,
	)
}
