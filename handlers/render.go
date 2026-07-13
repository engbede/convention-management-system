package handlers

import (
	"html/template"
	"log"
	"net/http"
)

func Render(
	w http.ResponseWriter,
	page string,
	data interface{},
) {

	var tmpl *template.Template

	if page == "form.html" ||
		page == "home.html" ||
		page == "login.html" ||
		page == "success.html" {

		tmpl = template.Must(
			template.ParseFiles(
				"templates/" + page,
			),
		)

	} else {

		tmpl = template.Must(
			template.ParseFiles(
				"templates/layouts/admin_layout.html",
				"templates/"+page,
			),
		)
	}

	if err := tmpl.ExecuteTemplate(
		w,
		page,
		data,
	); err != nil {

		log.Println("Template execution error:", err)
	}
}
