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

		tmpl = template.Must(
			template.ParseFiles(
				"templates/partials/header.html",
				"templates/partials/navbar.html",
				"templates/partials/avatar.html",
				"templates/partials/left_sidebar.html",
				"templates/partials/create_post.html",
				"templates/partials/post_card.html",
				"templates/partials/right_sidebar.html",
				"templates/partials/footer.html",
				"templates/partials/scripts.html",
				"templates/community.html",
			),
		)

	case "profile.html",
		"public_profile.html",
		"edit_profile.html",
		"notifications.html":

		tmpl = template.Must(
			template.ParseFiles(
				"templates/partials/header.html",
				"templates/partials/navbar.html",
				"templates/partials/avatar.html",
				"templates/partials/post_card.html",
				"templates/partials/footer.html",
				"templates/profile.html",
				"templates/public_profile.html",
				"templates/edit_profile.html",
				"templates/notifications.html",
			),
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
