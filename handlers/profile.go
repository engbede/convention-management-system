package handlers

import (
	"net/http"

	"convention-management-system/sessions"
)

func Profile(
	w http.ResponseWriter,
	r *http.Request,
) {

	session, _ := sessions.Store.Get(
		r,
		"youth-community",
	)

	data := map[string]any{
		"Username": session.Values["username"],
		"Role":     session.Values["role"],
	}

	Render(
		w,
		"profile.html",
		data,
	)
}
