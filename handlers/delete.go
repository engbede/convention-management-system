package handlers

import (
	"net/http"
	"strconv"

	"convention-management-system/repository"
)

func DeleteRegistration(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodPost {

		http.Redirect(
			w,
			r,
			"/registrations",
			http.StatusSeeOther,
		)

		return
	}

	id, err := strconv.Atoi(
		r.FormValue("id"),
	)

	if err != nil {

		http.Error(
			w,
			"Invalid registration ID",
			http.StatusBadRequest,
		)

		return
	}

	err = repository.DeleteRegistration(id)

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
		"/registrations",
		http.StatusSeeOther,
	)
}
