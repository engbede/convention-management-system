package handlers

import (
	"net/http"

	"convention-management-system/models"
)

func NewNotice(
	w http.ResponseWriter,
	r *http.Request,
) {

	data := struct {
		Title  string
		Action string
		Notice models.Notice
		Error  string
	}{
		Title:  "Create Notice",
		Action: "/notices/create",
	}

	err := Templates.ExecuteTemplate(
		w,
		"notice_form.html",
		data,
	)

	if err != nil {

		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)

		return
	}
}
