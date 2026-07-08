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

	err := Templates.ExecuteTemplate(
		w,
		"communication.html",
		data,
	)

	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
	}
}