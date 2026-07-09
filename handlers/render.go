package handlers

import (
	"html/template"
	"net/http"
)

func Render(
	w http.ResponseWriter,
	page string,
	data interface{},
) {

	tmpl := template.Must(
		template.ParseFiles(
			"templates/layouts/admin_layout.html",
			"templates/"+page,
		),
	)

	err := tmpl.ExecuteTemplate(
		w,
		page,
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