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

	var (
		tmpl *template.Template
		err  error
	)

	switch page {

	case "home.html",
		"form.html",
		"login.html",
		"success.html",
		"signup.html",
		"user login.html":

		tmpl, err = template.ParseFiles(
			"templates/" + page,
		)

	case "community.html":

		tmpl, err = template.ParseFiles(
			"templates/partials/header.html",
			"templates/partials/navbar.html",
			"templates/partials/footer.html",
			"templates/community.html",
		)

	default:

		tmpl, err = template.ParseFiles(
			"templates/layouts/admin_layout.html",
			"templates/partials/header.html",
			"templates/partials/footer.html",
			"templates/"+page,
		)

	}

	if err != nil {
		log.Println("Template parse error:", err)

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	err = tmpl.ExecuteTemplate(
		w,
		page,
		data,
	)

	if err != nil {

		log.Println("Template execution error:", err)

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
	}
}
