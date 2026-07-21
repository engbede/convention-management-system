package handlers

import "net/http"

func CommunityLogin(
	w http.ResponseWriter,
	r *http.Request,
) {
	http.Redirect(
		w,
		r,
		"/community",
		http.StatusSeeOther,
	)
}
