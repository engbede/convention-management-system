package handlers

import (
	"net/http"

	"convention-management-system/models"
	"convention-management-system/repository"
)

func ContactPage(
	w http.ResponseWriter,
	r *http.Request,
) {

	data := struct {
		Title string
		Query map[string]string
	}{
		Title: "Contact Us",
		Query: map[string]string{
			"success": r.URL.Query().Get("success"),
		},
	}

	Render(
		w,
		"contact.html",
		data,
	)
}

func SubmitInquiry(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {

		http.Redirect(
			w,
			r,
			"/contact",
			http.StatusSeeOther,
		)

		return
	}

	inquiry := models.Inquiry{

		Name: r.FormValue("name"),

		Phone: r.FormValue("phone"),

		Email: r.FormValue("email"),

		Subject: r.FormValue("subject"),

		Message: r.FormValue("message"),
	}

	err := repository.CreateInquiry(inquiry)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	http.Redirect(
		w,
		r,
		"/contact?success=1",
		http.StatusSeeOther,
	)
}

func ListInquiries(
	w http.ResponseWriter,
	r *http.Request,
) {

	inquiries, err := repository.GetAllInquiries()

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	Render(
		w,
		"inquiries.html",
		struct {
			Title     string
			Inquiries interface{}
		}{
			Title:     "Contact Inquiries",
			Inquiries: inquiries,
		},
	)
}
