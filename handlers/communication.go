package handlers

import "net/http"

func Communication(
	w http.ResponseWriter,
	r *http.Request,
) {

	data := struct {
		Title string
	}{
		Title: "Communication Centre",
	}

	Render(
		w,
		"communication.html",
		data,
	)
}